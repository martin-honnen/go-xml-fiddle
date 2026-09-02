importScripts("wasm_exec.js");

const go = new Go();
WebAssembly.instantiateStreaming(fetch("main.wasm"), go.importObject).then((result) => {
    // Start the Go runtime. main.go registers `greet` on the worker global
    // object through syscall/js; it is not a WebAssembly instance export.
    go.run(result.instance);
    postMessage({ action: "ready", payload: null });
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
        default:
            throw (`unknown action '${action}'`);
    }
};
