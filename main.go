package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
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

const jsonParserStylesheet = `<xsl:stylesheet version="3.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform"
  xmlns:xs="http://www.w3.org/2001/XMLSchema"
  exclude-result-prefixes="#all">
  <xsl:output method="adaptive"/>
  <xsl:param name="json" as="xs:string"/>
  <xsl:template name="xsl:initial-template">
    <xsl:sequence select="parse-json($json)"/>
  </xsl:template>
</xsl:stylesheet>`

var (
	jsonParser, jsonParserErr = compileJSONParser()
)

// compileJSONParser builds the parse-json wrapper once. Each evaluation only
// supplies the JSON text as its top-level parameter.
func compileJSONParser() (*xslt.Stylesheet, error) {
	tree, err := xdm.ParseString(jsonParserStylesheet, xdm.ParseOptions{})
	if err != nil {
		return nil, fmt.Errorf("parse JSON stylesheet: %w", err)
	}
	stylesheet, err := xslt.Compile(tree.Root, xslt.CompileOptions{})
	if err != nil {
		return nil, fmt.Errorf("compile JSON parser: %w", err)
	}
	return stylesheet, nil
}

// parseJSON turns one JSON text into its XDM item using the stylesheet
// compiled at startup, so all three evaluation modes share fn:parse-json.
func parseJSON(input string) (xdm.Sequence, error) {
	if jsonParserErr != nil {
		return nil, jsonParserErr
	}
	result, err := jsonParser.Transform(context.Background(), nil, xslt.TransformOptions{
		Params: map[string]xdm.Sequence{
			"json": xdm.One(xdm.NewString(input)),
		},
	})
	if err != nil {
		return nil, err
	}
	return result.Nodes, nil
}

// The labels the workbench lists alongside the hrefs of the documents an
// xsl:result-document named. They are bracketed in asterisks so that nothing
// a stylesheet could write as an href collides with one.
const (
	labelPrimary   = "*** primary result ***"
	labelSecondary = "*** secondary result ***"
	labelMessages  = "*** messages ***"
	labelWarnings  = "*** warnings ***"
	labelError     = "*** error ***"
)

// nsXHTML is the namespace whose html element selects the xhtml output method.
const nsXHTML = "http://www.w3.org/1999/xhtml"

const (
	nsXPathFunctions = "http://www.w3.org/2005/xpath-functions"
	nsXPathMap       = nsXPathFunctions + "/map"
	nsXPathArray     = nsXPathFunctions + "/array"
	nsXPathMath      = nsXPathFunctions + "/math"
	nsXQTErrors      = "http://www.w3.org/2005/xqt-errors"
)

// xpathNamespaces binds the standard XPath 3.1 prefixes at compile time.
// Prefixes in an XPath expression must be resolved before evaluation.
type xpathNamespaces struct{}

func (xpathNamespaces) ResolvePrefix(prefix string) (string, bool) {
	switch prefix {
	case "fn":
		return nsXPathFunctions, true
	case "map":
		return nsXPathMap, true
	case "array":
		return nsXPathArray, true
	case "math":
		return nsXPathMath, true
	case "err":
		return nsXQTErrors, true
	default:
		return "", false
	}
}

func (xpathNamespaces) DefaultElementNamespace() string { return "" }

func (xpathNamespaces) DefaultFunctionNamespace() string { return nsXPathFunctions }

// resultDoc is one entry in the list the workbench renders: the label for its
// dropdown, the serialized text, and the output method it was written with,
// which is what selects the editor's syntax mode.
type resultDoc struct {
	uri     string
	content string
	method  string
}

// jsResults hands the list to JavaScript. postMessage structured-clones it, so
// what crosses the boundary has to be plain arrays and objects.
func jsResults(docs ...resultDoc) any {
	out := make([]any, 0, len(docs))
	for _, d := range docs {
		out = append(out, map[string]any{
			"uri":     d.uri,
			"content": d.content,
			"method":  d.method,
		})
	}
	return out
}

// errorResult reports a failure in the same shape as a success, so that the
// JavaScript side has one message format to handle rather than two.
func errorResult(format string, args ...any) any {
	return jsResults(resultDoc{
		uri:     labelError,
		content: fmt.Sprintf(format, args...),
		method:  "text",
	})
}

// serializeTo renders one result, keeping whatever was written when
// serialization fails part way: a document that is well-formed up to the item
// the serializer refused is more use to the author than the error alone.
func serializeTo(write func(io.Writer) error) string {
	var buf bytes.Buffer
	if err := write(&buf); err != nil {
		if buf.Len() == 0 {
			return fmt.Sprintf("serialization error: %v", err)
		}
		return fmt.Sprintf("%s\n\nserialization error: %v", buf.String(), err)
	}
	return buf.String()
}

// effectiveMethod is the serialization method a result was actually written
// with, which is what the workbench maps to an editor mode.
//
// An xsl:output with no @method does not simply mean XML: a result whose first
// element is html is written as HTML instead, or as XHTML when that element is
// in the XHTML namespace, and DocBook's output is exactly that case. go-xml
// applies the rule in its serializer but does not export it, so it is followed
// here as section 9.1 states it. Getting it wrong would cost only syntax
// highlighting, which is why repeating it is worth the few lines.
func effectiveMethod(declared xslt.OutputSettings, seq xdm.Sequence) string {
	if m := strings.ToLower(declared.Method); m != "" {
		return m
	}
	first := firstResultElement(seq)
	if first == nil {
		return "xml"
	}
	switch {
	case first.Name.URI == nsXHTML && first.Name.Local == "html":
		// A 1.0 stylesheet whose result tree was built implicitly keeps the
		// XML method, which is the backwards-compatibility case go-xml
		// records in this flag.
		if declared.Version10Implicit {
			return "xml"
		}
		return "xhtml"
	case first.Name.URI == "" && strings.EqualFold(first.Name.Local, "html"):
		return "html"
	}
	return "xml"
}

// firstResultElement returns the element the default output method is chosen
// from, or nil when there is none to choose from.
//
// Whitespace ahead of the first element is skipped, and anything else rules
// out the HTML methods: they describe a document rather than an arbitrary
// sequence.
func firstResultElement(seq xdm.Sequence) *xdm.Node {
	var first *xdm.Node
	// The walk stops on the first element, and also on the first item that
	// rules one out — returning false for both, so that a document node whose
	// children rule it out does not let the search continue past it.
	var scan func(xdm.Sequence) bool
	scan = func(items xdm.Sequence) bool {
		for _, item := range items {
			switch v := item.(type) {
			case *xdm.Node:
				switch v.Kind {
				case xdm.KindDocument:
					kids := make(xdm.Sequence, 0, len(v.Children))
					for _, c := range v.Children {
						kids = append(kids, c)
					}
					if !scan(kids) {
						return false
					}
				case xdm.KindElement:
					if first == nil {
						first = v
					}
					return false
				case xdm.KindText:
					if !xdm.IsXMLWhitespace(v.Value) {
						return false
					}
				}
			case *xdm.Atomic:
				if !xdm.IsXMLWhitespace(v.String()) {
					return false
				}
			}
		}
		return true
	}
	scan(seq)
	return first
}

// setClock supplies the one timestamp fn:current-dateTime, fn:current-date and
// fn:current-time report.
//
// They read it from the dynamic context rather than the system clock, so that
// every call within one evaluation agrees — and go-xml leaves it unset in a
// Context built by hand, raising FODC0001 ("no transform clock configured")
// instead of inventing a reading. xslt.Transform defaults it to time.Now for
// the caller, which is why the same expression works in a stylesheet and fails
// in a bare query.
//
// The reading itself is UTC as far as the result is concerned: the offset comes
// from Context.ImplicitTimezone, whose zero value is Z, and not from the Go
// time's own location. That matches what a transformation reports here, since
// xslt.TransformOptions.ImplicitTimezone is left at its zero value too.
func setClock(ctx *xpath.Context) {
	ctx.Now, ctx.HasNow = time.Now(), true
}

func xpathEval(this js.Value, args []js.Value) interface{} {
	if len(args) == 0 || args[0].Type() != js.TypeString {
		return "xpath error: an XPath expression is required"
	}

	// Compile defaults to XPath 2.0. fn:sort is an XPath 3.1 function,
	// so explicitly select XPath 3.1 here.
	compiled, err := xpath.CompileVersion(args[0].String(), xpathNamespaces{}, xpath.XPath31)
	if err != nil {
		return fmt.Sprintf("xpath error: %v", err)
	}

	inputType, exprBase, sourceBase := stringArg(args, 2), stringArg(args, 3), stringArg(args, 4)
	resolver := newFetchResolver()

	var item xdm.Item
	if len(args) > 1 && args[1].Type() == js.TypeString {
		if inputType == "JSON" {
			seq, err := parseJSON(args[1].String())
			if err != nil {
				return fmt.Sprintf("xpath error: invalid JSON: %v", err)
			}
			if len(seq) != 1 {
				return "xpath error: parsing JSON did not produce one XDM item"
			}
			item = seq[0]
		} else {
			tree, err := xdm.ParseString(args[1].String(), xdm.ParseOptions{
				BaseURI:     sourceBase,
				DocumentURI: sourceBase,
			})
			if err != nil {
				return fmt.Sprintf("xpath error: invalid XML: %v", err)
			}
			item = tree.Root
			resolver.Preload(sourceBase, tree)
		}
	}

	ctx := xpath.NewContext(item, xpath.Builtins())
	ctx.Version = xpath.XPath31
	// The expression's own base URI, which is what a relative reference in
	// fn:doc resolves against and what fn:static-base-uri reports.
	ctx.StaticBaseURI = exprBase
	ctx.Docs = resolver
	ctx.Texts = resolver
	setClock(ctx)
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
//
// It returns an array of {uri, content, method} objects: the principal result
// first, then one for each document xsl:result-document produced, then the
// xsl:message output and the warnings if there were any. A failure is reported
// in the same shape, as a single entry labelled as an error.
func xslt30(this js.Value, args []js.Value) interface{} {
	if len(args) == 0 || args[0].Type() != js.TypeString {
		return errorResult("xslt error: a stylesheet is required")
	}

	inputType, sheetBase, sourceBase := stringArg(args, 2), stringArg(args, 3), stringArg(args, 4)
	resolver := newFetchResolver()

	stylesheetTree, err := xdm.ParseString(args[0].String(), xdm.ParseOptions{
		BaseURI:     sheetBase,
		DocumentURI: sheetBase,
		// $err:line-number in an xsl:catch has nothing else to report from.
		TrackPositions: true,
	})
	if err != nil {
		return errorResult("xslt error: invalid stylesheet XML: %v", err)
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
		return errorResult("xslt error: %v", err)
	}

	var source *xdm.Node
	var initialMatchSelection xdm.Sequence
	if len(args) > 1 && args[1].Type() == js.TypeString {
		if inputType == "JSON" {
			seq, err := parseJSON(args[1].String())
			if err != nil {
				return errorResult("xslt error: invalid source JSON: %v", err)
			}
			initialMatchSelection = seq
		} else {
			sourceTree, err := xdm.ParseString(args[1].String(), xdm.ParseOptions{
				BaseURI:     sourceBase,
				DocumentURI: sourceBase,
			})
			if err != nil {
				return errorResult("xslt error: invalid source XML: %v", err)
			}
			source = sourceTree.Root
			// So that fn:doc of this document's own URI hands back these very
			// nodes rather than a second parse of the same bytes.
			resolver.Preload(sourceBase, sourceTree)
		}
	}

	// The same resolver serves fn:doc and fn:unparsed-text. It is also how a
	// nested transform reaches its own xsl:import, which is what DocBook's
	// fn:transform pipeline stages need.
	result, err := stylesheet.Transform(context.Background(), source, xslt.TransformOptions{
		Documents:             resolver,
		Texts:                 resolver,
		InitialMatchSelection: initialMatchSelection,
	})
	if err != nil {
		// A failed transform returns no Result at all, so there are no
		// messages to show beside the error even though the stylesheet may
		// have produced some before it failed.
		return errorResult("xslt error: %v", err)
	}

	// The principal result comes first: it is what the workbench selects and
	// renders in its frame, and for a stylesheet that produces one document
	// it is the only entry.
	docs := []resultDoc{{
		uri:     labelPrimary,
		content: serializeTo(result.Serialize),
		method:  effectiveMethod(stylesheet.Output(), result.Nodes),
	}}
	// Each xsl:result-document is listed under the href the stylesheet wrote,
	// which is also the name the workbench saves it as. go-xml never writes
	// one to a destination itself, deliberately: what to do with an href is
	// the caller's decision.
	for i := range result.Secondary {
		sec := &result.Secondary[i]
		label := sec.Href
		if label == "" {
			// xsl:result-document may be written without an href, and then
			// there is no name to list it under.
			label = labelSecondary
		}
		docs = append(docs, resultDoc{
			uri: label,
			content: serializeTo(func(w io.Writer) error {
				// nil keeps the document's own character map, which was
				// resolved when it was produced.
				return sec.Serialize(w, nil)
			}),
			// Its own output settings, which is the point of @format: a
			// stylesheet may write XHTML chunks beside a JSON manifest.
			method: effectiveMethod(sec.Output, sec.Nodes),
		})
	}
	// xsl:message is how DocBook reports what its pipeline did, so it gets a
	// pane of its own rather than being dropped.
	if len(result.Messages) > 0 {
		docs = append(docs, resultDoc{
			uri:     labelMessages,
			content: strings.Join(result.Messages, "\n"),
			method:  "text",
		})
	}
	if len(result.Warnings) > 0 {
		docs = append(docs, resultDoc{
			uri:     labelWarnings,
			content: strings.Join(result.Warnings, "\n"),
			method:  "text",
		})
	}
	return jsResults(docs...)
}

// xquery31 compiles and evaluates an XQuery 3.1 query. The first argument is
// the query and the optional second argument is the context XML document.
func xquery31(this js.Value, args []js.Value) interface{} {
	if len(args) == 0 || args[0].Type() != js.TypeString {
		return "xquery error: a query is required"
	}

	inputType, queryBase, sourceBase := stringArg(args, 2), stringArg(args, 3), stringArg(args, 4)
	resolver := newFetchResolver()

	var item xdm.Item
	if len(args) > 1 && args[1].Type() == js.TypeString {
		if inputType == "JSON" {
			seq, err := parseJSON(args[1].String())
			if err != nil {
				return fmt.Sprintf("xquery error: invalid input JSON: %v", err)
			}
			if len(seq) != 1 {
				return "xquery error: parsing JSON did not produce one XDM item"
			}
			item = seq[0]
		} else {
			tree, err := xdm.ParseString(args[1].String(), xdm.ParseOptions{
				BaseURI:     sourceBase,
				DocumentURI: sourceBase,
			})
			if err != nil {
				return fmt.Sprintf("xquery error: invalid input XML: %v", err)
			}
			item = tree.Root
			resolver.Preload(sourceBase, tree)
		}
	}

	ctx := xpath.NewContext(item, xpath.Builtins())
	ctx.Version = xpath.XPath31
	ctx.Docs = resolver
	ctx.Texts = resolver
	setClock(ctx)
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
