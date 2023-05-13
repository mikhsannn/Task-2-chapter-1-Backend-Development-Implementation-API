package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
)

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the token from the Authorization header
		authHeader := c.GetHeader("Authorization")
		tokenString := strings.Split(authHeader, "Bearer ")[1]

		// Verify the token
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			// Check if the token signing method is HMAC
			_, ok := token.Method.(*jwt.SigningMethodHMAC)
			if !ok {
				return nil, nil
			}

			// Return the JWT secret key
			return []byte(os.Getenv("JWT_SECRET")), nil
		})
		if err != nil {
			log.Println("Failed to verify token:", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		// Check if the token is valid
		if _, ok := token.Claims.(jwt.Claims); !ok || !token.Valid {
			log.Println("Invalid token")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		// Token is valid, continue to the next middleware
		c.Next()
	}
}
