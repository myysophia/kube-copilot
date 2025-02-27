package tools

import (
	"fmt"
	"go.uber.org/zap"
	"strings"
)

var logger *zap.Logger

func init() {
	var err error
	logger, err = zap.NewProduction()
	if err != nil {
		panic(fmt.Sprintf("无法初始化日志: %v", err))
	}
}

// Tool 是一个接受输入并返回输出的函数类型
type Tool func(input string) (string, error)

// function call ，可以理解这里是hook点，可以在这里添加自己的工具
var CopilotTools = map[string]Tool{
	"search":  GoogleSearch,
	"python":  PythonREPL,
	"trivy":   Trivy,
	"kubectl": Kubectl,
	"jq":      JQ,
	"k8s_resource": SmartK8sResource,
}

// ToolPrompt 定义了与 LLM 交互的 JSON 格式
type ToolPrompt struct {
	Question string   `json:"question"` // 用户输入的问题
	Thought  string   `json:"thought"`  // AI 的思考过程
	Action   struct { // 需要执行的动作
		Name  string `json:"name"`  // 工具名称
		Input string `json:"input"` // 工具输入
	} `json:"action"`
	Observation string `json:"observation"`  // 工具执行结果
	FinalAnswer string `json:"final_answer"` // 最终答案
}

// SmartK8sResource 智能查询Kubernetes资源
// 输入: 资源名称（可以是模糊名称）
// 输出: 资源详细信息
func SmartK8sResource(input string) (string, error) {
	logger.Debug("调用智能K8s资源查询",
		zap.String("input", input),
	)
	
	// 清理输入
	input = strings.TrimSpace(input)
	
	// 检查是否是空输入
	if input == "" {
		return "请提供资源名称进行查询", nil
	}
	
	// 调用智能资源查询函数
	result, err := SmartResourceQuery(input)
	if err != nil {
		logger.Error("智能资源查询失败",
			zap.Error(err),
		)
		return fmt.Sprintf("查询失败: %v", err), err
	}
	
	// 如果结果为空，返回友好提示
	if strings.TrimSpace(result) == "" {
		return fmt.Sprintf("未找到与 '%s' 匹配的资源。请尝试使用更通用的名称或检查拼写。", input), nil
	}
	
	return result, nil
}
