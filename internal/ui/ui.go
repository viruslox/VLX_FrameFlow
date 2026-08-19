package ui

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed dist/*
var staticFS embed.FS

// ServeSPA serves the embedded SPA for a name-stripped request path. Paths that
// look like a file (contain a dot) are served as static assets with correct
// content types; everything else returns index.html for client-side routing.
func ServeSPA(c *gin.Context, rest string) {
	distFS, err := fs.Sub(staticFS, "dist")
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load frontend assets")
		return
	}

	if strings.Contains(rest, ".") {
		// Serve the asset from the stripped path so content types and 404s are
		// handled by the standard file server.
		c.Request.URL.Path = rest
		http.FileServer(http.FS(distFS)).ServeHTTP(c.Writer, c.Request)
		return
	}

	indexContent, err := fs.ReadFile(distFS, "index.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load index.html")
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", indexContent)
}

// ServeFrontend registers routes to serve the embedded Svelte SPA
func ServeFrontend(r *gin.Engine) {
	// Get a sub-filesystem rooted at the "dist" directory
	distFS, err := fs.Sub(staticFS, "dist")
	if err != nil {
		panic(err)
	}

	// Serve static files from the root path
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// If the requested path has a dot in it, assume it's a file request (like .js, .css, etc.)
		if strings.Contains(path, ".") {
			http.FileServer(http.FS(distFS)).ServeHTTP(c.Writer, c.Request)
			return
		}

		// Otherwise, serve index.html for SPA client-side routing
		indexContent, err := fs.ReadFile(distFS, "index.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to load index.html")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexContent)
	})
}
