#!/usr/bin/env bash
# GlyphDeck build script (bash)

set -e

echo "=== Building Go backend ==="
go build -o bin/glyphdeck ./cmd/glyphdeck

echo "=== Building frontend ==="
cd web
npm install
npm run build
cd ..

echo "=== Build complete ==="
echo "Backend: bin/glyphdeck"
echo "Frontend: web/dist/"
