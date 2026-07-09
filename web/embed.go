// Package web exposes the compiled frontend included in release binaries.
package web

import (
	"embed"
	"io/fs"
)

// compiledAssets contains the Vite build output. scripts/build.ps1 builds the
// frontend before Go compiles this package.
//
//go:embed dist
var compiledAssets embed.FS

// Assets returns the compiled frontend rooted at the Vite output directory.
func Assets() fs.FS {
	assets, err := fs.Sub(compiledAssets, "dist")
	if err != nil {
		panic("embedded frontend assets are unavailable: " + err.Error())
	}
	return assets
}
