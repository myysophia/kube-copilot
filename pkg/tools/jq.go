package tools

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"go.uber.org/zap"
)

// JQ 执行jq命令处理JSON数据
func JQ(input string) (string, error) {
	logger.Debug("准备执行 jq 命令",
		zap.String("input", input),
	)

	// 解析输入，格式应为: JSON数据 | jq表达式
	parts := strings.Split(input, "|")
	if len(parts) != 2 {
		return "", fmt.Errorf("输入格式错误，应为: JSON数据 | jq表达式")
	}

	jsonData := strings.TrimSpace(parts[0])
	jqExpr := strings.TrimSpace(parts[1])

	// 验证JSON数据是否有效
	var jsonObj interface{}
	if err := json.Unmarshal([]byte(jsonData), &jsonObj); err != nil {
		return "", fmt.Errorf("无效的JSON数据: %v", err)
	}

	// 创建临时文件存储JSON数据
	// 使用管道直接传递数据
	cmd := exec.Command("jq", jqExpr)
	cmd.Stdin = strings.NewReader(jsonData)

	// 执行jq命令
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

	// 不再截断输出
	// if len(result) > 8000 {
	//     result = result[:8000] + "\n...\n[输出已截断，总大小: " + fmt.Sprintf("%d", len(result)) + " 字节]"
	// }

	return strings.TrimSpace(string(output)), nil
}

// 智能处理JSON数据，提取特定字段
func processJSONWithJQ(jsonData string, query string) (string, error) {
	// 构建jq命令
	input := fmt.Sprintf("%s | %s", jsonData, query)
	return JQ(input)
} 