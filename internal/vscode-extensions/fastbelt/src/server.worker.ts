// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

// Hosts the WASM-compiled Go language server inside a worker. Go's
// `wasm_exec.js` runtime is inlined ahead of the worker bundles (esbuild
// banner) and defines `Go` on the global scope.
//
// The worker receives the WASM module bytes as its first message (the
// extension reads them and passes them in), then relays JSON-RPC messages
// between its message channel and the WASM server. The bytes are passed in
// rather than fetched here because a Web Worker is loaded from a blob/file URL
// it cannot resolve resources against.
//
// The language client exchanges JSON-RPC *objects* with the worker, while the
// Go server speaks bare JSON *strings* (jsonrpc2.RawFramer). This glue
// translates between the two representations.

declare const Go: {
    new(): {
        importObject: WebAssembly.Imports;
        run(instance: WebAssembly.Instance): Promise<void>
    }
};

// The message channel between the worker and the extension. It abstracts over
// Web Workers and Node.js worker threads.
export interface ServerPort {
    // Sends a message to the extension.
    postMessage(message: unknown): void;
    // Registers the receiver of the messages sent by the extension.
    onMessage(handler: (message: unknown) => void): void;
    // Ends the worker.
    close(): void;
}

// The first message sent to the worker. It carries the WASM module of the server.
export interface InitMessage {
    __init: true;
    wasm: Uint8Array<ArrayBuffer>;
}

export function runServer(port: ServerPort): void {
    const glue = globalThis as {
        wasmSend?: (message: string) => void;
        wasmReady?: () => void;
        wasmReceive?: (message: string) => void;
    };

    // Messages that arrive before the WASM server has registered its receive
    // callback are buffered and flushed once it signals readiness.
    const pending: string[] = [];
    let ready = false;
    let started = false;

    // Server -> client: the Go side hands us a JSON string; forward it as an object.
    glue.wasmSend = (message: string) => {
        port.postMessage(JSON.parse(message));
    };

    // Called by the Go side once `wasmReceive` is registered.
    glue.wasmReady = () => {
        ready = true;
        for (const message of pending) {
            glue.wasmReceive?.(message);
        }
        pending.length = 0;
    };

    port.onMessage((data) => {
        // The first message carries the WASM bytes and starts the server.
        if (!started && (data as Partial<InitMessage> | undefined)?.__init) {
            started = true;
            startServer((data as InitMessage).wasm).then(
                () => port.close(),
                (error) => {
                    console.error('Fastbelt language server failed:', error);
                    port.close();
                }
            );
            return;
        }
        // Client -> server: stringify the JSON-RPC object for the Go side.
        const message = JSON.stringify(data);
        if (ready && glue.wasmReceive) {
            glue.wasmReceive(message);
        } else {
            pending.push(message);
        }
    });
}

async function startServer(wasm: Uint8Array<ArrayBuffer>): Promise<void> {
    const go = new Go();
    const source = await WebAssembly.instantiate(wasm, go.importObject);
    // go.run resolves only when the Go program exits. The server blocks on its
    // connection, which keeps the WASM instance alive until the extension
    // terminates the worker.
    await go.run(source.instance);
}
