package handlers

import (
	"net/http"
	"strings"

	"github.com/CptPie/SyncRate/models"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// GetUserContext extracts user information from gin context and returns template data
func GetUserContext(c *gin.Context) gin.H {
	isAuth, _ := c.Get("is_authenticated")
	username, _ := c.Get("username")
	userID, _ := c.Get("user_id")

	return gin.H{
		"is_authenticated": isAuth,
		"username":         username,
		"user_id":          userID,
	}
}

func GetLogin(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)

		// Check if user is already logged in
		if userID := session.Get("user_id"); userID != nil {
			c.Redirect(http.StatusFound, "/")
			return
		}

		data := GetUserContext(c)
		data["title"] = "SyncRate | Login"
		c.HTML(http.StatusOK, "login.html", data)
	}
}

func PostLogin(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)

		username := strings.TrimSpace(c.PostForm("username"))
		password := c.PostForm("password")

		// Validate input
		if username == "" || password == "" {
			data := GetUserContext(c)
			data["title"] = "SyncRate | Login"
			data["error"] = "Username and password are required"
			c.HTML(http.StatusBadRequest, "login.html", data)
			return
		}

		// Get user from database
		var user models.User
		if err := db.Where("username = ?", username).First(&user).Error; err != nil {
			data := GetUserContext(c)
			data["title"] = "SyncRate | Login"
			data["error"] = "Invalid username or password"
			c.HTML(http.StatusUnauthorized, "login.html", data)
			return
		}

		// Check password
		err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
		if err != nil {
			data := GetUserContext(c)
			data["title"] = "SyncRate | Login"
			data["error"] = "Invalid username or password"
			c.HTML(http.StatusUnauthorized, "login.html", data)
			return
		}

		// Set session
		session.Set("user_id", user.UserID)
		session.Set("username", user.Username)
		if err := session.Save(); err != nil {
			data := GetUserContext(c)
			data["title"] = "SyncRate | Login"
			data["error"] = "Failed to create session"
			c.HTML(http.StatusInternalServerError, "login.html", data)
			return
		}

		// Redirect to home page
		c.Redirect(http.StatusFound, "/")
	}
}

func GetRegister(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)

		// Check if user is already logged in
		if userID := session.Get("user_id"); userID != nil {
			c.Redirect(http.StatusFound, "/")
			return
		}

		data := GetUserContext(c)
		data["title"] = "SyncRate | Register"
		c.HTML(http.StatusOK, "register.html", data)
	}
}

func PostRegister(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		username := strings.TrimSpace(c.PostForm("username"))
		email := strings.TrimSpace(c.PostForm("email"))
		password := c.PostForm("password")
		confirmPassword := c.PostForm("confirm_password")

		// Validate input
		if username == "" || email == "" || password == "" || confirmPassword == "" {
			data := GetUserContext(c)
			data["title"] = "SyncRate | Register"
			data["error"] = "All fields are required"
			c.HTML(http.StatusBadRequest, "register.html", data)
			return
		}

		// Check password confirmation
		if password != confirmPassword {
			data := GetUserContext(c)
			data["title"] = "SyncRate | Register"
			data["error"] = "Passwords do not match"
			c.HTML(http.StatusBadRequest, "register.html", data)
			return
		}

		// Validate password length
		if len(password) < 6 {
			data := GetUserContext(c)
			data["title"] = "SyncRate | Register"
			data["error"] = "Password must be at least 6 characters long"
			c.HTML(http.StatusBadRequest, "register.html", data)
			return
		}

		// Check if username already exists
		var existingUser models.User
		if err := db.Where("username = ?", username).First(&existingUser).Error; err == nil {
			data := GetUserContext(c)
			data["title"] = "SyncRate | Register"
			data["error"] = "Username already exists"
			c.HTML(http.StatusBadRequest, "register.html", data)
			return
		}

		// Hash password
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			data := GetUserContext(c)
			data["title"] = "SyncRate | Register"
			data["error"] = "Failed to process registration"
			c.HTML(http.StatusInternalServerError, "register.html", data)
			return
		}

		// Create user
		user := models.User{
			Username:     username,
			Email:        email,
			PasswordHash: string(hashedPassword),
		}

		if err := db.Create(&user).Error; err != nil {
			data := GetUserContext(c)
			data["title"] = "SyncRate | Register"
			data["error"] = "Failed to create account"
			c.HTML(http.StatusInternalServerError, "register.html", data)
			return
		}

		// Auto-login after registration
		session := sessions.Default(c)
		session.Set("user_id", user.UserID)
		session.Set("username", user.Username)
		if err := session.Save(); err != nil {
			// Registration succeeded but login failed - redirect to login page
			c.Redirect(http.StatusFound, "/login")
			return
		}

		// Redirect to home page
		c.Redirect(http.StatusFound, "/")
	}
}

func PostLogout(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)

		// Clear session
		session.Clear()
		if err := session.Save(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to logout",
			})
			return
		}

		// Redirect to home page
		c.Redirect(http.StatusFound, "/")
	}
}