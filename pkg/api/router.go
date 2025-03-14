package api

import (
	"github.com/feiskyer/kube-copilot/pkg/handlers"
	"github.com/feiskyer/kube-copilot/pkg/middleware"
	"github.com/feiskyer/kube-copilot/pkg/utils"
	"github.com/gin-gonic/gin"
)

// Router 配置所有路由
func Router() *gin.Engine {
	r := gin.Default()

	// 配置 CORS
	r.Use(middleware.CORS())

	// 配置 logger 中间件
	r.Use(middleware.Logger())

	// 配置性能统计中间件
	r.Use(middleware.PerfStats())

	// 注入全局变量到上下文
	r.Use(func(c *gin.Context) {
		// 从配置中获取 JWT 密钥
		jwtKey := utils.GetConfig().GetString("jwt.key")
		if jwtKey == "" {
			jwtKey = "your-secret-key" // 默认密钥，建议在生产环境中设置
		}
		c.Set("jwtKey", []byte(jwtKey))
		c.Next()
	})

	// 公开路由
	public := r.Group("")
	{
		public.GET("/version", handlers.Version)
		public.POST("/login", handlers.Login)
	}

	// API 路由组
	api := r.Group("/api")
	{
		// 诊断相关接口
		diagnose := api.Group("/diagnose")
		diagnose.Use(middleware.JWTAuth())
		{
			diagnose.POST("", handlers.Diagnose)
		}

		// 分析相关接口
		analyze := api.Group("/analyze")
		analyze.Use(middleware.JWTAuth())
		{
			analyze.POST("", handlers.Analyze)
		}

		// 执行相关接口
		execute := api.Group("/execute")
		execute.Use(middleware.JWTAuth())
		{
			execute.POST("", handlers.Execute)
		}

		// 性能统计接口
		perf := api.Group("/perf")
		perf.Use(middleware.JWTAuth())
		{
			perf.GET("/stats", handlers.PerfStats)
			perf.POST("/reset", handlers.ResetPerfStats)
		}
	}

	return r
}
