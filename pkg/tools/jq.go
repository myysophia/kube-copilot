package tools

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"go.uber.org/zap"
)

// JQ 执行jq命令处理JSON数据
// 功能特性：
// 1. 支持复杂的jq表达式
// 2. 自动验证JSON数据格式
// 3. 处理管道操作
// 参数：
//   - input: 输入格式为 "JSON数据 | jq表达式"
// 返回：
//   - string: jq处理后的结果
//   - error: 处理过程中的错误
func JQ(input string) (string, error) {
	logger.Debug("准备执行 jq 命令",
		zap.String("input", input),
	)

	// 解析输入，分离JSON数据和jq表达式
	parts := strings.Split(input, "|")
	if len(parts) != 2 {
		return "", fmt.Errorf("输入格式错误，应为: JSON数据 | jq表达式")
	}

	jsonData := strings.TrimSpace(parts[0])
	jqExpr := strings.TrimSpace(parts[1])

	// 验证JSON数据格式是否有效
	var jsonObj interface{}
	if err := json.Unmarshal([]byte(jsonData), &jsonObj); err != nil {
		return "", fmt.Errorf("无效的JSON数据: %v", err)
	}

	// 使用管道直接传递数据执行jq命令
	cmd := exec.Command("jq", jqExpr)
	cmd.Stdin = strings.NewReader(jsonData)

	// 执行命令并获取输出
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error("jq 命令执行失败",
			zap.Error(err),
			zap.String("output", string(output)),
		)
		return strings.TrimSpace(string(output)), err
	}

	logger.Debug("jq 命令执行成功",
		zap.String("output", string(output)),
	)

	return strings.TrimSpace(string(output)), nil
}

// processJSONWithJQ 智能处理JSON数据并提取特定字段
// 功能：
// 1. 自动构建jq查询表达式
// 2. 处理复杂的JSON结构
// 参数：
//   - jsonData: 原始JSON数据
//   - query: 要执行的jq查询
// 返回：
//   - string: 处理后的结果
//   - error: 处理过程中的错误
func processJSONWithJQ(jsonData string, query string) (string, error) {
	// 构建完整的jq命令输入
	input := fmt.Sprintf("%s | %s", jsonData, query)
	return JQ(input)
} 