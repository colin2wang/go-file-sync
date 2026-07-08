package web

import (
	"embed"
	"io/fs"
)

//go:embed dist/*
var staticFiles embed.FS

// StaticFiles returns the embedded static files filesystem.
func StaticFiles() (fs.FS, error) {
	// Try to get the dist subdirectory
	subFS, err := fs.Sub(staticFiles, "dist")
	if err != nil {
		// Fall back to root if dist doesn't exist
		return staticFiles, nil
	}
	return subFS, nil
}
