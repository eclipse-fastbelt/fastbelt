// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

// Builds the Fastbelt grammar language server as a WebAssembly module. The
// same module runs on every platform, so the extension ships no native binary.
// Go's `wasm_exec.js` runtime is inlined into the worker bundles by esbuild, so
// it is not copied here.

const { execFileSync } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');

const dist = path.join(__dirname, 'dist');
fs.mkdirSync(dist, { recursive: true });

// Compile the Go server for the JS/WASM target, without debug information.
execFileSync('go', ['build', '-trimpath', '-ldflags=-s -w', '-o', path.join(dist, 'server.wasm'), '../../grammar/server'], {
    cwd: __dirname,
    stdio: 'inherit',
    env: { ...process.env, GOOS: 'js', GOARCH: 'wasm' },
});

console.log('Built dist/server.wasm');
