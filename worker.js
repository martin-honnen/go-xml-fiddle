importScripts("wasm_exec.js");

const go = new Go();
WebAssembly.instantiateStreaming(fetch("main.wasm"), go.importObject).then((result) => {
    postMessage({action : 'hide', payload : { id : 'go-load-indicator' }});
    go.run(result.instance);
    postMessage({ action: "ready", payload: null });
    postMessage({action : 'hide', payload : { id : 'go-xml-load-indicator' }});

}).catch((err) => {
    console.error("Worker failed to load WASM module: ", err)
});

onmessage = ({ data }) => {
    const { action, payload } = data;
    switch (action) {
        case "greet":
            const { name } = payload;
            const res = self.greet(name);
            postMessage({ action: "result", payload: res });
            break;
        // The two base URIs are the base URI of the code and the base URI of
        // the input document. They are what a relative xsl:include,
        // xsl:import or fn:doc resolves against, so a stylesheet built from
        // modules -- DocBook, say -- cannot be compiled without them. Both
        // are optional: code typed into the editor has no base URI to give,
        // and the Go side treats a missing one as absent.
        case "xpath":
            const { xml, expression, inputType } = payload;
            const xpathRes = self.xpathEval(expression, xml,
                inputType,
                payload.codeBaseURI, payload.inputBaseURI);
            postMessage({ action: "result", payload: xpathRes });
            break;
        // A transformation answers with a list rather than a single string:
        // the principal result, then whatever xsl:result-document wrote, then
        // the xsl:message output and the warnings. It gets an action of its
        // own so the page can tell that shape apart from the single result
        // XPath and XQuery return.
        case "xslt":
            const xsltRes = self.xslt30(payload.xslt, payload.xml,
                payload.inputType,
                payload.codeBaseURI, payload.inputBaseURI);
            postMessage({ action: "XSLT-Results", payload: xsltRes });
            break;
        case "xquery":
            const xqueryRes = self.xquery31(payload.query, payload.xml,
                payload.inputType,
                payload.codeBaseURI, payload.inputBaseURI);
            postMessage({ action: "result", payload: xqueryRes });
            break;
        default:
            throw (`unknown action '${action}'`);
    }
};
