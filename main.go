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

	var root *xdm.Node
	if len(args) > 1 && args[1].Type() == js.TypeString {
		tree, err := xdm.ParseString(args[1].String(), xdm.ParseOptions{})
		if err != nil {
			return fmt.Sprintf("xpath error: invalid XML: %v", err)
		}
		root = tree.Root
	}

	ctx := xpath.NewContext(root, xpath.Builtins())
	ctx.Version = xpath.XPath31
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
// stylesheet XML and the optional second argument is the source XML.
func xslt30(this js.Value, args []js.Value) interface{} {
	if len(args) == 0 || args[0].Type() != js.TypeString {
		return "xslt error: a stylesheet is required"
	}

	stylesheetTree, err := xdm.ParseString(args[0].String(), xdm.ParseOptions{})
	if err != nil {
		return fmt.Sprintf("xslt error: invalid stylesheet XML: %v", err)
	}

	stylesheet, err := xslt.Compile(stylesheetTree.Root, xslt.CompileOptions{})
	if err != nil {
		return fmt.Sprintf("xslt error: %v", err)
	}

	var source *xdm.Node
	if len(args) > 1 && args[1].Type() == js.TypeString {
		sourceTree, err := xdm.ParseString(args[1].String(), xdm.ParseOptions{})
		if err != nil {
			return fmt.Sprintf("xslt error: invalid source XML: %v", err)
		}
		source = sourceTree.Root
	}

	result, err := stylesheet.Transform(context.Background(), source, xslt.TransformOptions{})
	if err != nil {
		return fmt.Sprintf("xslt error: %v", err)
	}

	return serializeSequence(result.Nodes, xslt.OutputSettings{Method: "adaptive", Indent: true})
}

// xquery31 compiles and evaluates an XQuery 3.1 query. The first argument is
// the query and the optional second argument is the context XML document.
func xquery31(this js.Value, args []js.Value) interface{} {
	if len(args) == 0 || args[0].Type() != js.TypeString {
		return "xquery error: a query is required"
	}

	var root *xdm.Node
	if len(args) > 1 && args[1].Type() == js.TypeString {
		tree, err := xdm.ParseString(args[1].String(), xdm.ParseOptions{})
		if err != nil {
			return fmt.Sprintf("xquery error: invalid input XML: %v", err)
		}
		root = tree.Root
	}

	ctx := xpath.NewContext(root, xpath.Builtins())
	ctx.Version = xpath.XPath31
	seq, err := xquery.Eval(args[0].String(), ctx, xquery.Options{})
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
