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

// ServeStatic serves the embedded Vue frontend.
func ServeStatic(r *gin.Engine) {
	dist, err := fs.Sub(distFS, "dist")
	if err != nil {
		// If dist doesn't exist (dev mode), skip static serving.
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