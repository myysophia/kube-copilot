package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/cobra"
)

var (
	// API server flags
	port   int
	jwtKey string
)

const (
	VERSION          = "v0.5.3"
	DEFAULT_USERNAME = "admin"
	DEFAULT_PASSWORD = "kube-copilot"
)

// JWT claims structure
type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// Request structures
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type DiagnoseRequest struct {
	Name      string `json:"name" binding:"required"`
	Namespace string `json:"namespace" binding:"required"`
}

type AnalyzeRequest struct {
	Resource string `json:"resource" binding:"required"`
}

type ExecuteRequest struct {
	Instructions string `json:"instructions" binding:"required"`
}

// JWT middleware
func jwtAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := c.GetHeader("Authorization")
		if tokenString == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Missing authorization token"})
			return
		}

		// Remove "Bearer " prefix if present
		if len(tokenString) > 7 && tokenString[:7] == "Bearer " {
			tokenString = tokenString[7:]
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return []byte(jwtKey), nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			return
		}

		c.Set("username", claims.Username)
		c.Next()
	}
}

// setupRouter configures the Gin router with all endpoints
func setupRouter() *gin.Engine {
	r := gin.Default()

	// 配置 CORS
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000"},
		AllowMethods:     []string{"GET", "POST", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "X-OpenAI-Key", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Version endpoint (unprotected)
	r.GET("/version", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"version": VERSION})
	})

	// Login endpoint
	r.POST("/login", func(c *gin.Context) {
		var req LoginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// 使用默认账户验证
		if req.Username != DEFAULT_USERNAME || req.Password != DEFAULT_PASSWORD {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}

		// Create JWT token
		claims := &Claims{
			Username: req.Username,
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
				NotBefore: jwt.NewNumericDate(time.Now()),
			},
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString([]byte(jwtKey))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not generate token"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"token": tokenString,
			"note": "Default credentials: admin/kube-copilot",
		})
	})

	// Protected endpoints
	protected := r.Group("")
	protected.Use(jwtAuth())
	{
		protected.POST("/diagnose", func(c *gin.Context) {
			var req DiagnoseRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			model := c.DefaultQuery("model", "gpt-4")
			cluster := c.DefaultQuery("cluster", "default")

			// TODO: Implement actual diagnosis using workflows.DiagnoseFlow
			result := fmt.Sprintf("Diagnosing pod %s in namespace %s using model %s on cluster %s", 
				req.Name, req.Namespace, model, cluster)

			c.JSON(http.StatusOK, gin.H{
				"message": result,
				"status": "success",
			})
		})

		protected.POST("/analyze", func(c *gin.Context) {
			var req AnalyzeRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			model := c.DefaultQuery("model", "gpt-4")
			cluster := c.DefaultQuery("cluster", "default")

			// TODO: Implement actual analysis
			result := fmt.Sprintf("Analyzing resource %s using model %s on cluster %s",
				req.Resource, model, cluster)

			c.JSON(http.StatusOK, gin.H{
				"message": result,
				"status": "success",
			})
		})

		protected.POST("/execute", func(c *gin.Context) {
			var req ExecuteRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			model := c.DefaultQuery("model", "gpt-4")
			cluster := c.DefaultQuery("cluster", "default")

			// TODO: Implement actual execution
			result := fmt.Sprintf("Executing instructions '%s' using model %s on cluster %s",
				req.Instructions, model, cluster)

			c.JSON(http.StatusOK, gin.H{
				"message": result,
				"status": "success",
			})
		})
	}

	return r
}

// serverCmd represents the server command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the API server",
	Run: func(cmd *cobra.Command, args []string) {
		// Validate required flags
		if jwtKey == "" {
			fmt.Println("Error: JWT key is required")
			os.Exit(1)
		}

		r := setupRouter()
		addr := fmt.Sprintf(":%d", port)
		fmt.Printf("Starting server on %s\n", addr)
		if err := r.Run(addr); err != nil {
			fmt.Printf("Error starting server: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	serverCmd.Flags().IntVarP(&port, "port", "p", 8080, "Port to run the server on")
	serverCmd.Flags().StringVar(&jwtKey, "jwt-key", "", "Key for signing JWT tokens")
	serverCmd.MarkFlagRequired("jwt-key")
	rootCmd.AddCommand(serverCmd)
} 