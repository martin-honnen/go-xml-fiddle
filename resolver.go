//go:build js && wasm

package main

import (
	"fmt"
	"strings"
	"sync"
	"syscall/js"

	"github.com/knroy/go-xml/xdm"
)

// fetchResolver loads stylesheet modules and documents by URI, using a
// synchronous XMLHttpRequest.
//
// Without one of these, go-xml disables xsl:include and xsl:import entirely —
// a nil CompileOptions.Resolver is documented as doing exactly that — and a
// stylesheet that imports its own modules is compiled as though those modules
// were empty. The failure is indirect: DocBook's docbook.xsl loses the import
// chain that declares its static variables, so the first use-when naming one
// reports XPST0008 rather than anything about a module.
//
// Synchronous is deliberate, and is why this only works on a worker thread.
// ResolveModule is a plain function call made from the middle of a compile, so
// it has to have the bytes before it returns; sync XHR is the only HTTP API
// that can do that, and the main thread is the one place the platform forbids
// it. fetch() cannot serve here at all — awaiting a promise needs the JS event
// loop to turn, and the Go stack holding the compiler is what would have to
// yield for that to happen.
//
// There is no confinement to mirror xslt.FileResolver's Roots: the browser's
// own origin policy is the boundary here, so a module is reachable only from
// where the page could fetch it anyway. What is enforced is the scheme; see
// resolveURI.
type fetchResolver struct {
	mu    sync.Mutex
	cache map[string]*xdm.Tree
}

// resolverCacheMax bounds the parsed documents one resolver retains. DocBook
// pulls in some sixty modules plus its locale files, so the bound is well
// above what a large stylesheet needs; reaching it costs a refetch.
const resolverCacheMax = 1024

func newFetchResolver() *fetchResolver {
	return &fetchResolver{cache: map[string]*xdm.Tree{}}
}

// ResolveModule implements xslt.ModuleResolver for xsl:include and xsl:import.
func (r *fetchResolver) ResolveModule(href, base string) (*xdm.Node, string, error) {
	uri, err := resolveURI(href, base)
	if err != nil {
		return nil, "", err
	}
	// A stylesheet module keeps its source positions; see loadTracked.
	tree, err := r.loadTracked(uri, true)
	if err != nil {
		return nil, "", err
	}
	return tree.Root, uri, nil
}

// ResolveDocument implements xpath.DocumentResolver for fn:doc and
// fn:document.
//
// Handing the same object to TransformOptions.Documents is also what lets a
// nested transform reach its own imports: go-xml looks for a ModuleResolver
// behind the document resolver, which is how DocBook's fn:transform pipeline
// stages compile.
func (r *fetchResolver) ResolveDocument(uri, base string) (*xdm.Tree, error) {
	resolved, err := resolveURI(uri, base)
	if err != nil {
		return nil, err
	}
	return r.loadTracked(resolved, false)
}

// ResolveText implements xpath.TextResolver for fn:unparsed-text.
func (r *fetchResolver) ResolveText(uri, base, encoding string) (string, error) {
	resolved, err := resolveURI(uri, base)
	if err != nil {
		return "", err
	}
	return httpGet(resolved, encoding)
}

// Preload records a tree that has already been parsed as the answer for uri.
//
// The worker parses the source document itself, from the editor's text, so
// without this a fn:doc naming that same URI would fetch and parse it a second
// time and hand back different nodes — and XSLT 2.0 section 16.1 requires two
// retrievals of one absolute URI to return the same node. An unresolvable uri
// is a no-op, which is the ordinary case for text typed into the editor.
func (r *fetchResolver) Preload(uri string, tree *xdm.Tree) {
	if tree == nil {
		return
	}
	resolved, err := resolveURI(uri, "")
	if err != nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cache[resolved] = tree
}

// loadTracked fetches and parses a URI, caching the result, and says whether
// the parsed tree should remember where each element was written.
//
// The cache is for correctness as much as for speed. fn:doc must return the
// same node for the same URI within one execution, so re-parsing would break
// node identity — and the compiler asks for each module more than once, since
// static analysis, import numbering and the ordinary walk each resolve the
// same href.
//
// Positions are kept for stylesheet modules and not for source documents. A
// module needs them because XSLT 3.0 section 8.3 publishes the line an error
// was raised on as $err:line-number, and nothing else can answer.
func (r *fetchResolver) loadTracked(uri string, trackPos bool) (*xdm.Tree, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if t, ok := r.cache[uri]; ok {
		// A cached tree serves a request that does not need more than it
		// has. Only a module asking for positions of a tree parsed without
		// them has to go back to the network.
		if !trackPos || t.HasPositions() {
			return t, nil
		}
	}
	body, err := httpGet(uri, "")
	if err != nil {
		return nil, err
	}
	tree, err := xdm.ParseString(body, xdm.ParseOptions{
		// Stamped on every element of the module, and what fn:document,
		// fn:resolve-uri and fn:static-base-uri resolve against. This is what
		// makes a nested relative href work: modules/variable.xsl including a
		// sibling resolves it against modules/, not against the main
		// stylesheet.
		BaseURI: uri,
		// This document *was* retrieved by URI, so it has a dm:document-uri
		// as well as a base URI, and fn:document-uri must report it.
		DocumentURI:    uri,
		TrackPositions: trackPos,
	})
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", uri, err)
	}
	// Cleared wholesale rather than evicted one at a time, as go-xml's own
	// resolver does: there is no useful recency signal here.
	if len(r.cache) >= resolverCacheMax {
		r.cache = map[string]*xdm.Tree{}
	}
	r.cache[uri] = tree
	return tree, nil
}

// resolveURI turns a reference into the absolute http(s) URI to fetch.
func resolveURI(href, base string) (string, error) {
	// An empty URI reference denotes the document the reference appears in,
	// so with a base in hand it resolves to that base rather than being an
	// error. This is what makes doc('') return the stylesheet itself, which
	// XSLT 2.0 section 16.1 requires.
	if href == "" {
		if base == "" {
			return "", fmt.Errorf("empty URI reference and no base URI")
		}
		href = base
	}
	abs, err := absoluteURI(href, base)
	if err != nil {
		return "", err
	}
	// A fragment identifier selects within a document rather than naming a
	// different one, so it is not part of the name of the resource to fetch.
	if i := strings.IndexByte(abs, '#'); i >= 0 {
		abs = abs[:i]
	}
	scheme := ""
	if i := strings.IndexByte(abs, ':'); i > 0 {
		scheme = strings.ToLower(abs[:i])
	}
	// Only the two schemes the workbench itself is served over. The
	// alternative is not a wider grant but a worse error message: the
	// browser refuses a file: or data: read from a worker anyway, and it
	// refuses it as an opaque network failure rather than as the thing it is.
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("cannot retrieve %q: only http and https URIs "+
			"can be read from the browser (resolved against base %q)", abs, base)
	}
	return abs, nil
}

// absoluteURI resolves href against base using the platform's own URL parser,
// rather than a second implementation of RFC 3986 in Go.
func absoluteURI(href, base string) (abs string, err error) {
	// The URL constructor throws on a reference it cannot resolve — a
	// relative href against the 'urn:from-string' base the workbench uses
	// for text typed into the editor is the case that matters, since a urn:
	// is not hierarchical and nothing can be resolved against it. Turning
	// that into an error gets it reported as XTSE0165 against the offending
	// xsl:include, which is what the author needs to see.
	defer func() {
		if v := recover(); v != nil {
			err = fmt.Errorf("cannot resolve %q against base %q: %s",
				href, base, jsPanicText(v))
		}
	}()
	ctor := js.Global().Get("URL")
	if ctor.IsUndefined() {
		return "", fmt.Errorf("no URL constructor in this environment")
	}
	if base == "" {
		return ctor.New(href).Get("href").String(), nil
	}
	return ctor.New(href, base).Get("href").String(), nil
}

// httpGet reads a URI synchronously. charset, when given, overrides whatever
// the response declares, which is how fn:unparsed-text's encoding argument is
// honoured.
func httpGet(uri, charset string) (body string, err error) {
	defer func() {
		if v := recover(); v != nil {
			err = fmt.Errorf("fetching %s: %s", uri, jsPanicText(v))
		}
	}()
	ctor := js.Global().Get("XMLHttpRequest")
	if ctor.IsUndefined() {
		return "", fmt.Errorf("fetching %s: no XMLHttpRequest in this "+
			"environment (module loading needs a worker)", uri)
	}
	xhr := ctor.New()
	// The false is the point: async=false blocks until the response is in
	// hand, which is the only shape ResolveModule can use.
	xhr.Call("open", "GET", uri, false)
	if charset != "" {
		xhr.Call("overrideMimeType", "text/plain; charset="+charset)
	}
	xhr.Call("send")
	status := xhr.Get("status").Int()
	// A non-2xx status is a failed retrieval even though the request itself
	// succeeded: an HTML 404 page parsed as a stylesheet module reports a
	// baffling well-formedness error instead of the missing file.
	if status != 0 && (status < 200 || status >= 300) {
		return "", fmt.Errorf("fetching %s: HTTP %d %s",
			uri, status, xhr.Get("statusText").String())
	}
	text := xhr.Get("responseText").String()
	if status == 0 && text == "" {
		return "", fmt.Errorf("fetching %s: request failed (network error, "+
			"or blocked by the origin policy)", uri)
	}
	return text, nil
}

// jsPanicText renders the value a failed js.Value call panicked with.
func jsPanicText(v any) string {
	switch e := v.(type) {
	case js.Error:
		return e.Error()
	case *js.Error:
		return e.Error()
	case error:
		return e.Error()
	}
	return fmt.Sprint(v)
}
