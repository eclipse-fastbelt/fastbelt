// Copyright 2025 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

import * as vscode from 'vscode';
import { Worker } from 'node:worker_threads';
import type { MessageTransports, ServerOptions } from 'vscode-languageclient/node.js';
import { LanguageClient, PortMessageReader, PortMessageWriter } from 'vscode-languageclient/node.js';
import { clientId, clientName, createClientOptions, nodeWorkerFile, registerFileSystem, wasmFile } from './client.js';

let client: LanguageClient | undefined;
let worker: Worker | undefined;

// This function is called when the extension is activated in a Node.js
// extension host (desktop or remote). The language server runs as a
// WebAssembly module inside a worker thread, so the same build serves every
// platform.
export async function activate(context: vscode.ExtensionContext): Promise<void> {
    client = await startLanguageClient(context);
}

// This function is called when the extension is deactivated.
export async function deactivate(): Promise<void> {
    try {
        await client?.stop();
    } finally {
        await worker?.terminate();
    }
}

async function startLanguageClient(context: vscode.ExtensionContext): Promise<LanguageClient> {
    const dist = vscode.Uri.joinPath(context.extensionUri, 'dist');

    // Called on start and whenever the client restarts the server.
    const serverOptions: ServerOptions = async (): Promise<MessageTransports> => {
        await worker?.terminate();
        const wasm = await vscode.workspace.fs.readFile(vscode.Uri.joinPath(dist, wasmFile));
        const workerFileUri = vscode.Uri.joinPath(dist, nodeWorkerFile)
        // Wrap the worker file URI in a URL object for the Worker constructor.
        // Otherwise, the Worker constructor may not correctly resolve the file path.
        const serverWorker = new Worker(new URL(workerFileUri.toString()));
        worker = serverWorker;
        // The worker starts the server with the WASM bytes of its first message.
        serverWorker.postMessage({ __init: true, wasm });
        // The worker acts as a message port for the LSP transports.
        return {
            reader: new PortMessageReader(serverWorker),
            writer: new PortMessageWriter(serverWorker)
        };
    };

    const client = new LanguageClient(
        clientId,
        clientName,
        serverOptions,
        createClientOptions()
    );
    registerFileSystem(client);

    // Start the client. This will also launch the server
    await client.start();
    return client;
}
