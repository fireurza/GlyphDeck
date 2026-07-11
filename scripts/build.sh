#!/usr/bin/env bash
# GlyphDeck build script (bash)

set -e

echo "=== Building frontend ==="
cd web
npm ci
npm run build
cd ..

echo "=== Building Go backend ==="
go build -o bin/glyphdeck ./cmd/glyphdeck

echo "=== Build complete ==="
echo "Binary: bin/glyphdeck"
