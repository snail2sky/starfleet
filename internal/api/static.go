package api

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed all:dist
var distFS embed.FS

// setCacheHeaders applies cache policy for the embedded SPA.
// Hashed assets (/assets/*) are immutable and cached for a year; index.html
// and the SPA fallback are never cached — embedded files have no validators
// (zero modtime), so without explicit headers browsers keep serving stale
// index.html that points at chunk names from an older build.
func setCacheHeaders(c *gin.Context, path string) {
	if strings.HasPrefix(path, "/assets/") {
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
		return
	}
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
}

// ServeStatic serves the embedded Vue frontend.
func ServeStatic(r *gin.Engine) {
	dist, err := fs.Sub(distFS, "dist")
	if err != nil {
		return
	}

	// Fresh clones only contain .gitkeep — the UI is added by `make frontend`.
	// Tell the user instead of serving a directory listing.
	if _, statErr := dist.Open("index.html"); statErr != nil {
		r.NoRoute(func(c *gin.Context) {
			setCacheHeaders(c, c.Request.URL.Path)
			path := c.Request.URL.Path
			if strings.HasPrefix(path, "/api") || strings.HasPrefix(path, "/health") {
				c.JSON(404, gin.H{"error": "not found"})
				return
			}
			c.String(http.StatusServiceUnavailable,
				"starfleet web UI is not embedded in this binary.\n"+
					"Build it with: make frontend && go build ./cmd/starfleet/\n"+
					"API is available under /api/v1/\n")
		})
		return
	}

	fileServer := http.FileServer(http.FS(dist))

	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		// Serve API routes normally.
		if strings.HasPrefix(path, "/api") || strings.HasPrefix(path, "/health") {
			c.JSON(404, gin.H{"error": "not found"})
			return
		}
		setCacheHeaders(c, path)
		// Try to serve static file.
		f, err := dist.Open(strings.TrimPrefix(path, "/"))
		if err != nil {
			// SPA fallback: serve index.html for client-side routing.
			c.FileFromFS("/", http.FS(dist))
			return
		}
		f.Close()
		fileServer.ServeHTTP(c.Writer, c.Request)
	})
}
