package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func GetLogin(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.HTML(http.StatusOK, "login.html", gin.H{
			"title": "Login",
		})
	}
}

func PostLogin(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: Implement login logic
		c.JSON(http.StatusNotImplemented, gin.H{
			"error": "Login not implemented yet",
		})
	}
}

func GetRegister(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.HTML(http.StatusOK, "register.html", gin.H{
			"title": "Register",
		})
	}
}

func PostRegister(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: Implement registration logic
		c.JSON(http.StatusNotImplemented, gin.H{
			"error": "Registration not implemented yet",
		})
	}
}

func PostLogout(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: Implement logout logic
		c.JSON(http.StatusNotImplemented, gin.H{
			"error": "Logout not implemented yet",
		})
	}
}