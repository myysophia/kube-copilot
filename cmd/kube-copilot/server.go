package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/sashabaranov/go-openai"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/feiskyer/kube-copilot/pkg/assistants"
	"github.com/feiskyer/kube-copilot/pkg/utils"
)

var (
	// API server flags
	port   int
	jwtKey string
	logger *zap.Logger

	// Execute flags (从 execute.go 同步)
	maxTokens     = 20480
	countTokens   = true
	verbose       = true
	maxIterations = 10
)

const (
	VERSION          = "v0.1.8"
	DEFAULT_USERNAME = "admin"
	DEFAULT_PASSWORD = "novastar"
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
	Instructions   string   `json:"instructions" binding:"required"`
	Args           string   `json:"args" binding:"required"`
	Provider       string   `json:"provider"`
	BaseUrl        string   `json:"baseUrl"`
	CurrentModel   string   `json:"currentModel"`
	Cluster        string   `json:"cluster"`
	SelectedModels []string `json:"selectedModels"`
}

type AIResponse struct {
	Question string `json:"question"`
	Thought  string `json:"thought"`
	Action   struct {
		Name  string `json:"name"`
		Input string `json:"input"`
	} `json:"action"`
	Observation string `json:"observation"`
	FinalAnswer string `json:"final_answer"`
}

// initLogger 初始化 Zap 日志配置
func initLogger() {
	// 使用新的日志工具包初始化日志
	logConfig := utils.DefaultLogConfig()
	// 设置日志级别为 Debug
	logConfig.Level = zapcore.DebugLevel
	
	var err error
	logger, err = utils.InitLogger(logConfig)
	if err != nil {
		panic(fmt.Sprintf("初始化日志失败: %v", err))
	}

	// 初始化性能统计工具
	perfStats := utils.GetPerfStats()
	perfStats.SetLogger(logger)
	perfStats.SetEnableLogging(true)

	logger.Info("日志系统初始化完成",
		zap.String("log_dir", logConfig.LogDir),
		zap.String("log_file", logConfig.Filename),
		zap.Int("max_size_mb", logConfig.MaxSize),
		zap.Int("max_backups", logConfig.MaxBackups),
		zap.Int("max_age_days", logConfig.MaxAge),
	)
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
	verbose = true
	r := gin.Default()

	// 配置 CORS
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-OpenAI-Key", "X-API-Key", "X-Requested-With"},
		ExposeHeaders:    []string{"Content-Length", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
		AllowWildcard:    true,
		AllowWebSockets:  true,
	}))

	// 添加请求日志中间件
	r.Use(func(c *gin.Context) {
		// 请求开始时间
		startTime := time.Now()

		// 读取请求体
		var bodyBytes []byte
		if c.Request.Body != nil {
			bodyBytes, _ = c.GetRawData()
			// 将请求体放回，以便后续中间件使用
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		logger.Debug("收到请求",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("body", string(bodyBytes)),
		)

		// 开始请求性能统计
		perfStats := utils.GetPerfStats()
		perfStats.StartTimer("http_request_" + c.Request.Method + "_" + c.Request.URL.Path)

		// 处理请求
		c.Next()

		// 请求结束时间
		duration := time.Since(startTime)

		// 停止请求性能统计
		elapsed := perfStats.StopTimer("http_request_" + c.Request.Method + "_" + c.Request.URL.Path)

		logger.Debug("请求处理完成",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("duration", duration),
			zap.Duration("perf_duration", elapsed),
		)
	})

	// 全局处理 OPTIONS 请求
	r.OPTIONS("/*path", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

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
			"note":  "Default credentials: admin/novastar",
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

			model := c.DefaultQuery("model", "gpt-4o")
			cluster := c.DefaultQuery("cluster", "default")

			// TODO: Implement actual diagnosis using workflows.DiagnoseFlow
			result := fmt.Sprintf("Diagnosing pod %s in namespace %s using model %s on cluster %s",
				req.Name, req.Namespace, model, cluster)

			c.JSON(http.StatusOK, gin.H{
				"message": result,
				"status":  "success",
			})
		})

		protected.POST("/analyze", func(c *gin.Context) {
			var req AnalyzeRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			model := c.DefaultQuery("model", "gpt-4o")
			cluster := c.DefaultQuery("cluster", "default")

			// TODO: Implement actual analysis
			result := fmt.Sprintf("Analyzing resource %s using model %s on cluster %s",
				req.Resource, model, cluster)

			c.JSON(http.StatusOK, gin.H{
				"message": result,
				"status":  "success",
			})
		})

		protected.POST("/execute", func(c *gin.Context) {
			// 获取性能统计工具
			perfStats := utils.GetPerfStats()
			// 开始整体执行计时
			defer perfStats.TraceFunc("execute_total")()

			// 获取API Key，这是调用OpenAI API所必需的
			apiKey := c.GetHeader("X-API-Key")
			if apiKey == "" {
				logger.Error("缺少 API Key")
				c.JSON(http.StatusBadRequest, gin.H{"error": "Missing API Key"})
				return
			}

			// 解析请求体中的执行指令
			var req ExecuteRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				logger.Debug("Execute 请求解析失败",
					zap.Error(err),
				)
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("请求格式错误: %v", err)})
				return
			}

			// 记录请求信息（不包含敏感数据）
			logger.Debug("Execute 接口收到请求",
				zap.String("instructions", req.Instructions),
				zap.String("args", req.Args),
				zap.String("provider", req.Provider),
				zap.String("baseUrl", req.BaseUrl),
				zap.String("currentModel", req.CurrentModel),
				zap.Strings("selectedModels", req.SelectedModels),
				zap.String("cluster", req.Cluster),
				zap.String("apiKey", "***"), // 安全考虑，不记录实际的API Key
			)

			// 确定使用的模型，如果请求中未指定则使用默认值
			executeModel := req.CurrentModel
			if executeModel == "" {
				executeModel = "gpt-4"
			}

			// 构建执行指令，合并instructions和args
			instructions := req.Instructions
			if req.Args != "" && !strings.Contains(instructions, req.Args) {
				instructions = fmt.Sprintf("%s %s", req.Instructions, req.Args)
			}

			// 清理指令中可能包含的路径前缀
			cleanInstructions := strings.TrimPrefix(instructions, "/execute")
			cleanInstructions = strings.TrimSpace(cleanInstructions)

			logger.Debug("Execute 执行参数",
				zap.String("model", executeModel),
				zap.String("instructions", cleanInstructions),
				zap.String("baseUrl", req.BaseUrl),
				zap.String("cluster", req.Cluster),
			)

			// 构建发送给OpenAI的消息
			messages := []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: executeSystemPrompt_cn, // 使用中文系统提示
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: cleanInstructions,
				},
			}

			// 开始AI助手执行计时
			perfStats.StartTimer("execute_assistant")

			// 调用AI助手执行指令
			response, _, err := assistants.AssistantWithConfig(executeModel, messages, maxTokens, countTokens, verbose, maxIterations, apiKey, req.BaseUrl)

			// 停止AI助手执行计时
			assistantDuration := perfStats.StopTimer("execute_assistant")
			logger.Info("AI助手执行完成",
				zap.Duration("duration", assistantDuration),
			)

			if err != nil {
				logger.Error("Execute 执行失败",
					zap.Error(err),
				)
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": fmt.Sprintf("执行失败: %v", err),
				})
				return
			}

			// 开始响应解析计时
			perfStats.StartTimer("execute_response_parse")
			
			// 解析AI助手的响应
			var aiResp AIResponse
			
			// 首先尝试标准JSON解析
			err = json.Unmarshal([]byte(response), &aiResp)
			
			// 如果标准解析失败，尝试更健壮的解析方法
			if err != nil {
				logger.Debug("标准JSON解析失败，尝试更健壮的解析方法",
					zap.Error(err),
					zap.String("response", response),
				)
				
				// 尝试使用工具函数提取final_answer
				finalAnswer, extractErr := utils.ExtractField(response, "final_answer")
				if extractErr == nil && finalAnswer != "" {
					logger.Debug("成功使用工具函数提取final_answer",
						zap.String("final_answer", finalAnswer),
					)
					
					// 停止响应解析计时
					parseDuration := perfStats.StopTimer("execute_response_parse")
					logger.Debug("响应解析完成（工具函数提取）",
						zap.Duration("duration", parseDuration),
					)
					
					// 返回提取的final_answer
					c.JSON(http.StatusOK, gin.H{
						"message": finalAnswer,
						"status":  "success",
					})
					return
				}
				
				// 如果提取失败，尝试清理JSON后再解析
				cleanedJSON := utils.CleanJSON(response)
				if err2 := json.Unmarshal([]byte(cleanedJSON), &aiResp); err2 == nil && aiResp.FinalAnswer != "" {
					logger.Debug("成功从清理后的JSON中提取final_answer",
						zap.String("final_answer", aiResp.FinalAnswer),
					)
					
					// 停止响应解析计时
					parseDuration := perfStats.StopTimer("execute_response_parse")
					logger.Debug("响应解析完成（清理JSON后解析）",
						zap.Duration("duration", parseDuration),
					)
					
					// 返回提取的final_answer
					c.JSON(http.StatusOK, gin.H{
						"message": aiResp.FinalAnswer,
						"status":  "success",
					})
					return
				}
				
				// 如果所有方法都失败，尝试从非标准JSON中提取
				var genericResp map[string]interface{}
				if err2 := json.Unmarshal([]byte(response), &genericResp); err2 == nil {
					if finalAnswer, ok := genericResp["final_answer"].(string); ok && finalAnswer != "" {
						logger.Debug("成功从非标准JSON中提取final_answer",
							zap.String("final_answer", finalAnswer),
						)
						
						// 停止响应解析计时
						parseDuration := perfStats.StopTimer("execute_response_parse")
						logger.Debug("响应解析完成（非标准JSON提取）",
							zap.Duration("duration", parseDuration),
						)
						
						// 返回清理后的响应，只包含final_answer
						cleanResp := map[string]interface{}{
							"message": finalAnswer,
							"status":  "success",
						}
						c.JSON(http.StatusOK, cleanResp)
						return
					}
				}

				// 停止响应解析计时
				parseDuration := perfStats.StopTimer("execute_response_parse")
				logger.Debug("所有解析方法均失败，返回原始响应",
					zap.Duration("duration", parseDuration),
				)
				
				// 如果无法提取final_answer，返回完整响应
				c.JSON(http.StatusOK, gin.H{
					"message": response,
					"status":  "success",
				})
				return
			}

			// 停止响应解析计时
			parseDuration := perfStats.StopTimer("execute_response_parse")
			logger.Debug("响应解析完成（标准格式）",
				zap.Duration("duration", parseDuration),
			)
			
			// 处理标准JSON响应
			if aiResp.FinalAnswer != "" {
				// 返回最终答案
				c.JSON(http.StatusOK, gin.H{
					"message": aiResp.FinalAnswer,
					"status":  "success",
				})
			} else {
				// 如果没有最终答案，返回处理中的状态
				c.JSON(http.StatusOK, gin.H{
					"message": "指令正在执行中，请稍候...",
					"status":  "processing",
				})
			}
		})

		// 添加性能统计API
		protected.GET("/perf/stats", func(c *gin.Context) {
			perfStats := utils.GetPerfStats()
			stats := perfStats.PrintStats()
			c.JSON(http.StatusOK, gin.H{
				"stats": stats,
			})
		})

		// 重置性能统计API
		protected.POST("/perf/reset", func(c *gin.Context) {
			perfStats := utils.GetPerfStats()
			perfStats.ResetMetrics()
			c.JSON(http.StatusOK, gin.H{
				"message": "性能统计已重置",
				"status":  "success",
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
		// 初始化日志
		initLogger()
		defer logger.Sync()

		logger.Info("启动服务器",
			zap.Int("port", port),
		)

		// Validate required flags
		if jwtKey == "" {
			logger.Fatal("缺少必要参数: jwt-key")
		}

		r := setupRouter()
		addr := fmt.Sprintf(":%d", port)

		logger.Info("服务器开始监听",
			zap.String("address", addr),
		)

		if err := r.Run(addr); err != nil {
			logger.Fatal("服务器启动失败",
				zap.Error(err),
			)
		}
	},
}

func init() {
	serverCmd.Flags().IntVarP(&port, "port", "p", 8080, "Port to run the server on")
	serverCmd.Flags().StringVar(&jwtKey, "jwt-key", "", "Key for signing JWT tokens")
	serverCmd.MarkFlagRequired("jwt-key")
	rootCmd.AddCommand(serverCmd)
}
