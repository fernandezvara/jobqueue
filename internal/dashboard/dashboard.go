package dashboard

import (
	"embed"
	"io/fs"
)

//go:embed static/*
var content embed.FS

// GetFileSystem returns a fs.FS that contains the dashboard static files
func GetFileSystem() fs.FS {
	fsys, err := fs.Sub(content, "static")
	if err != nil {
		panic(err)
	}
	return fsys
}
