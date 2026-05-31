package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/CptPie/SyncRate/models"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// resetTokenTTL is how long an admin-generated reset link stays valid.
const resetTokenTTL = time.Hour

// generateResetToken returns a high-entropy raw token (handed to the user) and
// its SHA-256 hash (stored in the DB).
func generateResetToken() (raw string, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	raw = hex.EncodeToString(b)
	return raw, hashResetToken(raw), nil
}

// hashResetToken hashes a raw token for storage/lookup. SHA-256 is appropriate
// here (not bcrypt) because the token is already 256 bits of randomness and we
// need to look it up by hash.
func hashResetToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// lookupValidResetToken resolves a raw token to its record and user, returning
// ok=false if the token is unknown, already used, or expired.
func lookupValidResetToken(db *gorm.DB, raw string) (models.PasswordResetToken, models.User, bool) {
	var token models.PasswordResetToken
	if raw == "" {
		return token, models.User{}, false
	}
	if err := db.Where("token_hash = ?", hashResetToken(raw)).First(&token).Error; err != nil {
		return token, models.User{}, false
	}
	if token.UsedAt != nil || time.Now().After(token.ExpiresAt) {
		return token, models.User{}, false
	}

	var user models.User
	if err := db.First(&user, token.UserID).Error; err != nil {
		return token, models.User{}, false
	}
	return token, user, true
}

// buildResetURL produces the absolute link an admin hands to the user. It
// prefers APP_BASE_URL and otherwise derives scheme+host from the request.
func buildResetURL(c *gin.Context, rawToken string) string {
	base := strings.TrimRight(os.Getenv("APP_BASE_URL"), "/")
	if base == "" {
		scheme := "http"
		if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
			scheme = "https"
		}
		base = scheme + "://" + c.Request.Host
	}
	return base + "/reset/" + rawToken
}

// PostAdminResetPassword (admin-only) issues a fresh reset link for a user,
// invalidating any of that user's outstanding unused tokens first.
func PostAdminResetPassword(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		idParam := c.Param("id")
		id, err := strconv.ParseUint(idParam, 10, 32)
		if err != nil {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"title": "SyncRate | Error",
				"error": "Invalid user ID: " + err.Error(),
			})
			return
		}

		var user models.User
		if err := db.First(&user, uint(id)).Error; err != nil {
			c.HTML(http.StatusNotFound, "error.html", gin.H{
				"title": "SyncRate | Error",
				"error": "User not found",
			})
			return
		}

		raw, hash, err := generateResetToken()
		if err != nil {
			log.Printf("PostAdminResetPassword: token generation failed: %v", err)
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"title": "SyncRate | Error",
				"error": "Failed to generate reset link",
			})
			return
		}

		// Only the newest link should work.
		db.Where("user_id = ? AND used_at IS NULL", user.UserID).Delete(&models.PasswordResetToken{})

		token := models.PasswordResetToken{
			UserID:    user.UserID,
			TokenHash: hash,
			ExpiresAt: time.Now().Add(resetTokenTTL),
		}
		if err := db.Create(&token).Error; err != nil {
			log.Printf("PostAdminResetPassword: failed to store token: %v", err)
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"title": "SyncRate | Error",
				"error": "Failed to create reset link",
			})
			return
		}

		log.Printf("PostAdminResetPassword: issued reset link for user %d", user.UserID)

		data := GetUserContext(c)
		data["title"] = "SyncRate | Reset Link"
		data["resetUser"] = user.Username
		data["resetURL"] = buildResetURL(c, raw)
		data["expiresAt"] = token.ExpiresAt.Format("2006-01-02 15:04 MST")
		c.HTML(http.StatusOK, "admin-reset-link.html", data)
	}
}

// renderResetForm shows the public new-password form for a valid token.
func renderResetForm(c *gin.Context, status int, rawToken, username, errMsg string) {
	data := GetUserContext(c)
	data["title"] = "SyncRate | Reset Password"
	data["token"] = rawToken
	data["resetUsername"] = username
	if errMsg != "" {
		data["error"] = errMsg
	}
	c.HTML(status, "reset-password.html", data)
}

// GetResetPassword renders the new-password form if the token is valid.
func GetResetPassword(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := c.Param("token")
		_, user, ok := lookupValidResetToken(db, raw)
		if !ok {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"title": "SyncRate | Reset Password",
				"error": "This password reset link is invalid or has expired.",
			})
			return
		}
		renderResetForm(c, http.StatusOK, raw, user.Username, "")
	}
}

// PostResetPassword consumes a valid token and sets the new password.
func PostResetPassword(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := c.Param("token")
		token, user, ok := lookupValidResetToken(db, raw)
		if !ok {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"title": "SyncRate | Reset Password",
				"error": "This password reset link is invalid or has expired.",
			})
			return
		}

		password := c.PostForm("password")
		confirm := c.PostForm("confirm_password")

		switch {
		case password == "" || confirm == "":
			renderResetForm(c, http.StatusBadRequest, raw, user.Username, "Both password fields are required")
			return
		case password != confirm:
			renderResetForm(c, http.StatusBadRequest, raw, user.Username, "Passwords do not match")
			return
		case len(password) < 6:
			renderResetForm(c, http.StatusBadRequest, raw, user.Username, "Password must be at least 6 characters long")
			return
		}

		hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			log.Printf("PostResetPassword: hashing failed: %v", err)
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"title": "SyncRate | Error",
				"error": "Failed to reset password",
			})
			return
		}

		now := time.Now()
		tx := db.Begin()
		if err := tx.Model(&models.User{}).Where("user_id = ?", user.UserID).
			Update("password_hash", string(hashed)).Error; err != nil {
			tx.Rollback()
			log.Printf("PostResetPassword: failed to update password: %v", err)
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"title": "SyncRate | Error",
				"error": "Failed to reset password",
			})
			return
		}
		// Mark this token used and drop any other outstanding tokens.
		token.UsedAt = &now
		if err := tx.Save(&token).Error; err != nil {
			tx.Rollback()
			log.Printf("PostResetPassword: failed to mark token used: %v", err)
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"title": "SyncRate | Error",
				"error": "Failed to reset password",
			})
			return
		}
		tx.Where("user_id = ? AND used_at IS NULL", user.UserID).Delete(&models.PasswordResetToken{})
		tx.Commit()

		log.Printf("PostResetPassword: password reset for user %d", user.UserID)

		data := GetUserContext(c)
		data["title"] = "SyncRate | Login"
		data["success"] = "Your password has been reset. Please log in with your new password."
		c.HTML(http.StatusOK, "login.html", data)
	}
}
