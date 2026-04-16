package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed dist
var distFS embed.FS

// DistFileSystem returns a http.FileSystem rooted at the dist/ directory.
// Used by the API server to serve the Vue frontend.
func DistFileSystem() http.FileSystem {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic("web: failed to sub dist: " + err.Error())
	}
	return http.FS(sub)
}
