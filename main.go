package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"syscall/js"
	"time"

	"github.com/knroy/go-xml/xdm"
	"github.com/knroy/go-xml/xpath"
	"github.com/knroy/go-xml/xquery"
	"github.com/knroy/go-xml/xslt"
)

func main() {
	// Create a channel to keep the program running
	c := make(chan struct{}, 0)

	// Register a function to be called from JavaScript. Keep the Func alive
	// for as long as the WebAssembly program is running.
	greetFunc := js.FuncOf(greet)
	defer greetFunc.Release()
	js.Global().Set("greet", greetFunc)

	xpathEvalFunc := js.FuncOf(xpathEval)
	defer xpathEvalFunc.Release()
	js.Global().Set("xpathEval", xpathEvalFunc)

	xslt30Func := js.FuncOf(xslt30)
	defer xslt30Func.Release()
	js.Global().Set("xslt30", xslt30Func)

	xquery31Func := js.FuncOf(xquery31)
	defer xquery31Func.Release()
	js.Global().Set("xquery31", xquery31Func)

	fmt.Println("Go WebAssembly initialized")

	// Keep the program alive
	<-c
}

// greet is called through the JavaScript interop boundary. String values are
// converted to and from js.Value at that boundary.
func greet(this js.Value, args []js.Value) interface{} {
	name := ""
	if len(args) > 0 && args[0].Type() == js.TypeString {
		name = args[0].String()
	}
	if name == "" {
		name = "World"
	}
	message := fmt.Sprintf("Hello, %s! From Go WebAssembly at %s", name, time.Now())
	return message
}

// stringArg returns args[i] when it is a string, and "" otherwise, so that a
// caller may omit the trailing base-URI arguments.
func stringArg(args []js.Value, i int) string {
	if len(args) > i && args[i].Type() == js.TypeString {
		return args[i].String()
	}
	return ""
}

func xpathEval(this js.Value, args []js.Value) interface{} {
	if len(args) == 0 || args[0].Type() != js.TypeString {
		return "xpath error: an XPath expression is required"
	}

	// Compile defaults to XPath 2.0. fn:sort is an XPath 3.1 function,
	// so explicitly select XPath 3.1 here.
	compiled, err := xpath.CompileVersion(args[0].String(), nil, xpath.XPath31)
	if err != nil {
		return fmt.Sprintf("xpath error: %v", err)
	}

	exprBase, sourceBase := stringArg(args, 2), stringArg(args, 3)
	resolver := newFetchResolver()

	var root *xdm.Node
	if len(args) > 1 && args[1].Type() == js.TypeString {
		tree, err := xdm.ParseString(args[1].String(), xdm.ParseOptions{
			BaseURI:     sourceBase,
			DocumentURI: sourceBase,
		})
		if err != nil {
			return fmt.Sprintf("xpath error: invalid XML: %v", err)
		}
		root = tree.Root
		resolver.Preload(sourceBase, tree)
	}

	ctx := xpath.NewContext(root, xpath.Builtins())
	ctx.Version = xpath.XPath31
	// The expression's own base URI, which is what a relative reference in
	// fn:doc resolves against and what fn:static-base-uri reports.
	ctx.StaticBaseURI = exprBase
	ctx.Docs = resolver
	ctx.Texts = resolver
	seq, err := compiled.Eval(ctx)

	if err != nil {
		return fmt.Sprintf("xpath error: %v", err)
	}

	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)

	xslt.Serialize(writer, seq, xslt.OutputSettings{Method: "adaptive", Indent: true}, nil)
	writer.Flush()
	return buf.String()
}

// xslt30 compiles and runs an XSLT 3.0 stylesheet. The first argument is the
// stylesheet XML and the optional second argument is the source XML. The
// optional third and fourth are the base URIs of those two documents, which is
// what xsl:include, xsl:import and fn:doc resolve their relative hrefs
// against; a stylesheet with no relative references needs neither.
func xslt30(this js.Value, args []js.Value) interface{} {
	if len(args) == 0 || args[0].Type() != js.TypeString {
		return "xslt error: a stylesheet is required"
	}

	sheetBase, sourceBase := stringArg(args, 2), stringArg(args, 3)
	resolver := newFetchResolver()

	stylesheetTree, err := xdm.ParseString(args[0].String(), xdm.ParseOptions{
		BaseURI:     sheetBase,
		DocumentURI: sheetBase,
		// $err:line-number in an xsl:catch has nothing else to report from.
		TrackPositions: true,
	})
	if err != nil {
		return fmt.Sprintf("xslt error: invalid stylesheet XML: %v", err)
	}
	resolver.Preload(sheetBase, stylesheetTree)

	// Without a Resolver, go-xml disables xsl:include and xsl:import — and
	// disables them silently, so a stylesheet built from modules compiles as
	// though every module were empty. BaseURI is what the top-level hrefs
	// resolve against; each module's own base URI comes from the resolver.
	stylesheet, err := xslt.Compile(stylesheetTree.Root, xslt.CompileOptions{
		Resolver: resolver,
		BaseURI:  sheetBase,
	})
	if err != nil {
		return fmt.Sprintf("xslt error: %v", err)
	}

	var source *xdm.Node
	if len(args) > 1 && args[1].Type() == js.TypeString {
		sourceTree, err := xdm.ParseString(args[1].String(), xdm.ParseOptions{
			BaseURI:     sourceBase,
			DocumentURI: sourceBase,
		})
		if err != nil {
			return fmt.Sprintf("xslt error: invalid source XML: %v", err)
		}
		source = sourceTree.Root
		// So that fn:doc of this document's own URI hands back these very
		// nodes rather than a second parse of the same bytes.
		resolver.Preload(sourceBase, sourceTree)
	}

	// The same resolver serves fn:doc and fn:unparsed-text. It is also how a
	// nested transform reaches its own xsl:import, which is what DocBook's
	// fn:transform pipeline stages need.
	result, err := stylesheet.Transform(context.Background(), source, xslt.TransformOptions{
		Documents: resolver,
		Texts:     resolver,
	})
	if err != nil {
		return fmt.Sprintf("xslt error: %v", err)
	}

	return result.String() //serializeSequence(result.Nodes, xslt.OutputSettings{Method: "adaptive", Indent: true})
}

// xquery31 compiles and evaluates an XQuery 3.1 query. The first argument is
// the query and the optional second argument is the context XML document.
func xquery31(this js.Value, args []js.Value) interface{} {
	if len(args) == 0 || args[0].Type() != js.TypeString {
		return "xquery error: a query is required"
	}

	queryBase, sourceBase := stringArg(args, 2), stringArg(args, 3)
	resolver := newFetchResolver()

	var root *xdm.Node
	if len(args) > 1 && args[1].Type() == js.TypeString {
		tree, err := xdm.ParseString(args[1].String(), xdm.ParseOptions{
			BaseURI:     sourceBase,
			DocumentURI: sourceBase,
		})
		if err != nil {
			return fmt.Sprintf("xquery error: invalid input XML: %v", err)
		}
		root = tree.Root
		resolver.Preload(sourceBase, tree)
	}

	ctx := xpath.NewContext(root, xpath.Builtins())
	ctx.Version = xpath.XPath31
	ctx.Docs = resolver
	ctx.Texts = resolver
	seq, err := xquery.Eval(args[0].String(), ctx, xquery.Options{
		// BaseURI is what the query runs under; DeclarationBaseURI is what a
		// relative "declare base-uri" in the prolog resolves against.
		BaseURI:            queryBase,
		DeclarationBaseURI: queryBase,
	})
	if err != nil {
		return fmt.Sprintf("xquery error: %v", err)
	}

	return serializeSequence(seq, xslt.OutputSettings{Method: "adaptive", Indent: true})
}

func serializeSequence(seq xdm.Sequence, settings xslt.OutputSettings) string {
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	if err := xslt.Serialize(writer, seq, settings, nil); err != nil {
		return fmt.Sprintf("serialization error: %v", err)
	}
	if err := writer.Flush(); err != nil {
		return fmt.Sprintf("serialization error: %v", err)
	}
	return buf.String()
}
