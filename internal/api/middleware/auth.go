package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// AuthConfig holds authentication configuration.
type AuthConfig struct {
	Enabled  bool
	Username string
	Password string // plain text (compared in constant time)
	Secret   string // JWT secret
}

// AuthState holds the live auth configuration so reloads take effect
// without restarting the server. Readers are lock-free.
type AuthState struct {
	v atomic.Pointer[AuthConfig]
}

// NewAuthState creates a state preloaded with cfg.
func NewAuthState(cfg AuthConfig) *AuthState {
	s := &AuthState{}
	s.v.Store(&cfg)
	return s
}

// Store replaces the active auth configuration (config reload).
func (s *AuthState) Store(cfg AuthConfig) {
	s.v.Store(&cfg)
}

// Load returns the active auth configuration.
func (s *AuthState) Load() AuthConfig {
	return *s.v.Load()
}

// JWTClaims represents JWT token claims.
type JWTClaims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// Authenticate accepts either HTTP Basic credentials or a Bearer JWT and
// rejects the request with a JSON 401 otherwise. It reads the live AuthState
// on every request so config reloads apply immediately. It deliberately does
// not send WWW-Authenticate so browsers never raise the native credentials
// dialog over the SPA login flow.
func Authenticate(state *AuthState) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := state.Load()
		if !cfg.Enabled {
			c.Next()
			return
		}
		if username, password, ok := c.Request.BasicAuth(); ok &&
			subtle.ConstantTimeCompare([]byte(username), []byte(cfg.Username)) == 1 &&
			subtle.ConstantTimeCompare([]byte(password), []byte(cfg.Password)) == 1 {
			c.Set("username", username)
			c.Next()
			return
		}
		if tokenStr := extractToken(c); tokenStr != "" {
			claims := &JWTClaims{}
			token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
				return []byte(cfg.Secret), nil
			})
			if err == nil && token.Valid {
				c.Set("username", claims.Username)
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
	}
}

// GenerateToken creates a JWT token for the given username.
func GenerateToken(cfg AuthConfig, username string, duration time.Duration) (string, error) {
	claims := JWTClaims{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(duration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "starfleet",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.Secret))
}

func extractToken(c *gin.Context) string {
	bearer := c.GetHeader("Authorization")
	if strings.HasPrefix(bearer, "Bearer ") {
		return strings.TrimPrefix(bearer, "Bearer ")
	}
	if token := c.Query("token"); token != "" {
		return token
	}
	return ""
}
