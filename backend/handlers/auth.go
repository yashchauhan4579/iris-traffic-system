package handlers

import (
	"log"
	"net/http"
	"os"
	"time"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/irisdrone/backend/database"
	"github.com/irisdrone/backend/models"
	"golang.org/x/crypto/bcrypt"
)

var jwtSecret = []byte(os.Getenv("JWT_SECRET"))

func init() {
	if len(jwtSecret) == 0 {
		jwtSecret = []byte("default-dev-secret-change-me")
	}
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type AuthResponse struct {
	Token string      `json:"token"`
	User  models.User `json:"user"`
}

// Login handles user authentication
func Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	var user models.User
	if err := database.DB.Where("username = ?", req.Username).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Generate JWT
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": user.ID,
		"exp": time.Now().Add(hashDuration()).Unix(),
	})

	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, AuthResponse{
		Token: tokenString,
		User:  user,
	})
}

// hashDuration returns standard duration (24h)
func hashDuration() time.Duration {
	return 24 * time.Hour
}

// SeedAdminUser ensures the default user exists
func SeedAdminUser() {
	username := "wiredleap_atcc"
	password := "wiredleap12**"
	
	var count int64
	database.DB.Model(&models.User{}).Where("username = ?", username).Count(&count)
	
	if count == 0 {
		hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			log.Printf("❌ Failed to hash password: %v", err)
			return
		}
		
		admin := models.User{
			Username:     username,
			PasswordHash: string(hashedBytes),
			Role:         "admin",
		}
		
		if err := database.DB.Create(&admin).Error; err != nil {
			log.Printf("❌ Failed to create admin user: %v", err)
		} else {
			log.Println("✅ Admin user seeded successfully")
		}
	}
}

// AuthMiddleware protects routes
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header format"})
			c.Abort()
			return
		}

		tokenString := parts[1]
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return jwtSecret, nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		c.Next()
	}
}
