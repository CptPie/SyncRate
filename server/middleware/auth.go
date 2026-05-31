package middleware

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strings"

	"github.com/CptPie/SyncRate/models"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// RequireAuth middleware checks if user is authenticated
func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		userID := session.Get("user_id")

		if userID == nil {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireAdmin middleware ensures the request comes from a logged-in user whose
// account has admin privileges. The admin flag is read authoritatively from the
// database on every request so a stale or demoted session can't retain access.
func RequireAdmin(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		userID := session.Get("user_id")

		if userID == nil {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		var user models.User
		if err := db.First(&user, userID).Error; err != nil || !user.IsAdmin {
			c.String(http.StatusForbidden, "403 Forbidden: admin access required")
			c.Abort()
			return
		}

		c.Set("is_admin", true)
		c.Next()
	}
}

// RequireAPIKey middleware protects programmatic API endpoints with a shared
// secret supplied via the API_KEY environment variable. The key may be sent as
// "Authorization: Bearer <key>" or in the "X-API-Key" header.
func RequireAPIKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		expected := os.Getenv("API_KEY")
		if expected == "" {
			// Fail closed: with no key configured the endpoint is unusable
			// rather than silently open.
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"error": "API access is not configured",
			})
			return
		}

		provided := c.GetHeader("X-API-Key")
		if provided == "" {
			if auth := c.GetHeader("Authorization"); strings.HasPrefix(auth, "Bearer ") {
				provided = strings.TrimPrefix(auth, "Bearer ")
			}
		}

		if subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid or missing API key",
			})
			return
		}

		c.Next()
	}
}

// SetUserContext middleware sets current user information in context
func SetUserContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		userID := session.Get("user_id")
		username := session.Get("username")
		isAdmin := session.Get("is_admin")

		if userID != nil && username != nil {
			c.Set("user_id", userID)
			c.Set("username", username)
			c.Set("is_authenticated", true)
			c.Set("is_admin", isAdmin == true)
		} else {
			c.Set("is_authenticated", false)
			c.Set("is_admin", false)
		}

		c.Next()
	}
}
