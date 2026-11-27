// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

// Entry point of the Node.js worker thread that hosts the language server in
// a desktop or remote extension host. See server.worker.ts for details.

import { parentPort } from 'node:worker_threads';
import { runServer } from './server.worker.js';

const port = parentPort;
if (!port) {
    throw new Error('The Fastbelt language server must run in a worker thread.');
}

runServer({
    postMessage: (message) => port.postMessage(message),
    onMessage: (handler) => {
        port.on('message', handler);
    },
    // Closing the port lets the worker thread end if the server exits.
    close: () => port.close(),
});
