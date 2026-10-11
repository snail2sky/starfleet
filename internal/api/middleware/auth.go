package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// AuthConfig holds authentication configuration.
type AuthConfig struct {
	Enabled  bool
	Username string
	Password string // bcrypt hash or plain (for basic auth)
	Secret   string // JWT secret
}

// JWTClaims represents JWT token claims.
type JWTClaims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// BasicAuth returns a middleware for HTTP Basic Authentication.
func BasicAuth(cfg AuthConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !cfg.Enabled {
			c.Next()
			return
		}
		username, password, ok := c.Request.BasicAuth()
		if !ok ||
			subtle.ConstantTimeCompare([]byte(username), []byte(cfg.Username)) != 1 ||
			subtle.ConstantTimeCompare([]byte(password), []byte(cfg.Password)) != 1 {
			c.Header("WWW-Authenticate", `Basic realm="Starfleet"`)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Set("username", username)
		c.Next()
	}
}

// JWTAuth returns a middleware for JWT token authentication.
func JWTAuth(cfg AuthConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !cfg.Enabled {
			c.Next()
			return
		}
		tokenStr := extractToken(c)
		if tokenStr == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			return
		}
		claims := &JWTClaims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
			return []byte(cfg.Secret), nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		c.Set("username", claims.Username)
		c.Next()
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
