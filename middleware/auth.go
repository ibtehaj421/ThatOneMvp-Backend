package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
	"health/anam/backend/models"
)

var JwtSecret = []byte("super-secret-key-change-this-in-production")

// Update: Now accepts and embeds the role
func GenerateToken(userID uint, email string, role string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"email":   email,
		"role":    role, 
		"exp":     time.Now().Add(time.Hour * 72).Unix(),
	})
	return token.SignedString(JwtSecret)
}

// DB is set by main after ConnectDB so the middleware can refresh roles.
var DB *gorm.DB

func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header missing"})
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return JwtSecret, nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		// Safe extraction — old tokens may be missing claims.
		userIDFloat, _ := claims["user_id"].(float64)
		userID := uint(userIDFloat)
		if userID == 0 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			return
		}

		// Always fetch the current role from the DB so stale tokens can't
		// carry a wrong or empty role claim.
		var user models.User
		if err := DB.Select("id", "email", "role").First(&user, userID).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			return
		}

		c.Set("user_id", userID)
		c.Set("email", user.Email)
		c.Set("role", string(user.Role))
		c.Next()
	}
}

// NEW: RBAC Middleware
func RequireRole(allowedRoles ...models.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRoleStr := c.GetString("role")
		userRole := models.Role(userRoleStr)

		isAllowed := false
		for _, role := range allowedRoles {
			if role == userRole {
				isAllowed = true
				break
			}
		}

		if !isAllowed {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Forbidden: You do not have the required permissions for this action",
			})
			return
		}

		c.Next()
	}
}