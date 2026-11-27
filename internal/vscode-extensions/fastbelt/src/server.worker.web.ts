// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

// Entry point of the Web Worker that hosts the language server in a browser
// extension host. See server.worker.ts for details.

import { runServer } from './server.worker.js';

const scope = self as unknown as {
    postMessage(message: unknown): void;
    onmessage: ((event: MessageEvent) => void) | null;
    close(): void;
};

runServer({
    postMessage: (message) => scope.postMessage(message),
    onMessage: (handler) => {
        scope.onmessage = (event) => handler(event.data);
    },
    close: () => scope.close(),
});
