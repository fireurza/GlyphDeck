#!/usr/bin/env bash
# GlyphDeck dev launcher (bash)
# Starts backend and frontend in separate terminals

echo "Starting GlyphDeck backend..."
gnome-terminal -- bash -c "go run ./cmd/glyphdeck; exec bash" 2>/dev/null || \
osascript -e 'tell app "Terminal" to do script "cd '"$(pwd)"' && go run ./cmd/glyphdeck"' 2>/dev/null || \
echo "Backend: go run ./cmd/glyphdeck (open in a new terminal)"

echo "Starting GlyphDeck frontend..."
gnome-terminal -- bash -c "cd web && npm run dev; exec bash" 2>/dev/null || \
osascript -e 'tell app "Terminal" to do script "cd '"$(pwd)"'/web && npm run dev"' 2>/dev/null || \
echo "Frontend: cd web && npm run dev (open in a new terminal)"

echo "GlyphDeck dev servers started."
