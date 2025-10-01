package middleware

import (
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
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

// SetUserContext middleware sets current user information in context
func SetUserContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		userID := session.Get("user_id")
		username := session.Get("username")

		if userID != nil && username != nil {
			c.Set("user_id", userID)
			c.Set("username", username)
			c.Set("is_authenticated", true)
		} else {
			c.Set("is_authenticated", false)
		}

		c.Next()
	}
}