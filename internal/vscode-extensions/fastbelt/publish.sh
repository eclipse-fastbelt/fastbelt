#!/usr/bin/env bash

set -euo pipefail

# Require tokens for both marketplaces before doing any work.
if [[ -z "${VSCE_PAT:-}" ]]; then
  echo "VSCE_PAT is required."
  exit 1
fi

if [[ -z "${OVSX_PAT:-}" ]]; then
  echo "OVSX_PAT is required."
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Derive the output file name from extension metadata.
PACKAGE_NAME="$(node -p "require('./package.json').name")"
PACKAGE_VERSION="$(node -p "require('./package.json').version")"

# The language server is built as a WebAssembly module, so one package serves
# all platforms, including VS Code for the Web. Keep a stable filename so the
# exact same artifact can be published to both VS Marketplace and Open VSX.
vsix_file="${PACKAGE_NAME}-${PACKAGE_VERSION}.vsix"

echo "==> Building and packaging ${vsix_file}"
# Rebuild from a clean dist directory.
rm -rf dist

npx vsce package --out "$vsix_file"

echo "==> Publishing ${vsix_file} to VS Marketplace"
npx --no-install vsce publish --packagePath "$vsix_file"

echo "==> Publishing ${vsix_file} to Open VSX"
npx --no-install ovsx publish "$vsix_file"

echo "Done publishing the extension."
