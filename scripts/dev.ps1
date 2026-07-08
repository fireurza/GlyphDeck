# GlyphDeck dev launcher (PowerShell)
# Starts backend and frontend in separate windows

Write-Host "Starting GlyphDeck backend..."
Start-Process pwsh -ArgumentList "-NoExit", "-Command", "go run ./cmd/glyphdeck"

Write-Host "Starting GlyphDeck frontend..."
Start-Process pwsh -ArgumentList "-NoExit", "-Command", "cd web; npm run dev"

Write-Host "GlyphDeck dev servers started."
