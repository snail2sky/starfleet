package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupCORS() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS())
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func TestCORSSameOriginAllowed(t *testing.T) {
	r := setupCORS()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Host = "starfleet.example"
	req.Header.Set("Origin", "http://starfleet.example")
	r.ServeHTTP(w, req)
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://starfleet.example" {
		t.Errorf("expected origin echo, got %q", got)
	}
}

// TestCORSEvilOriginRejected: arbitrary sites must not get CORS access
// (with auth disabled they could otherwise read data from any visitor's
// local starfleet).
func TestCORSEvilOriginRejected(t *testing.T) {
	r := setupCORS()
	for _, origin := range []string{"https://evil.com", "http://starfleet.example.evil.com", "null"} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.Host = "starfleet.example"
		req.Header.Set("Origin", origin)
		r.ServeHTTP(w, req)
		if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("origin %q must not be allowed, got ACAO=%q", origin, got)
		}
	}
}

func TestCORSNoOriginHeader(t *testing.T) {
	r := setupCORS()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("non-browser request must pass, got %d", w.Code)
	}
}
