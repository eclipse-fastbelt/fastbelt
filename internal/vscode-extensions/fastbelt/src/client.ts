// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

// Language client parts shared by the desktop and the browser extension.

import * as vscode from 'vscode';
import type { BaseLanguageClient, LanguageClientOptions } from 'vscode-languageclient';

export const clientId = 'fastbelt';
export const clientName = 'Fastbelt';

// The bundled files of the language server.
export const wasmFile = 'server.wasm';
export const nodeWorkerFile = 'server.worker.node.js';
export const webWorkerFile = 'server.worker.web.js';

export function createClientOptions(): LanguageClientOptions {
    return {
        // No scheme restriction: the server reads files through the client, so
        // documents of virtual workspaces and untitled documents work as well.
        documentSelector: [{ language: 'fastbelt' }],
        outputChannel: vscode.window.createOutputChannel(
            'Fastbelt Grammar Language Server'
        )
    };
}

interface FileSystemParams {
    uri: string;
}

// The language server runs as WebAssembly in a worker and cannot access files
// itself. It requests them from the client instead, which serves them from the
// VS Code file system API. That works for local, remote, and virtual
// workspaces alike. The methods and their types are defined by the Go package
// typefox.dev/fastbelt/internal/grammar (ClientFileSystem).
export function registerFileSystem(client: BaseLanguageClient): void {
    client.onRequest('fastbelt/fs/stat', async (params: FileSystemParams) => {
        const stat = await vscode.workspace.fs.stat(vscode.Uri.parse(params.uri));
        return { type: stat.type };
    });
    client.onRequest('fastbelt/fs/readFile', async (params: FileSystemParams) => {
        const content = await vscode.workspace.fs.readFile(vscode.Uri.parse(params.uri));
        return { content: toBase64(content) };
    });
    client.onRequest('fastbelt/fs/readDirectory', async (params: FileSystemParams) => {
        const entries = await vscode.workspace.fs.readDirectory(vscode.Uri.parse(params.uri));
        return { entries: entries.map(([name, type]) => ({ name, type })) };
    });
}

// Encodes bytes as base64 with the means available in both Node.js and browsers.
function toBase64(bytes: Uint8Array): string {
    let binary = '';
    const chunkSize = 0x8000;
    for (let i = 0; i < bytes.length; i += chunkSize) {
        binary += String.fromCharCode(...bytes.subarray(i, i + chunkSize));
    }
    return btoa(binary);
}
