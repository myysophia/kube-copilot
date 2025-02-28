package tools

import (
	"go.uber.org/zap"
	"os/exec"
	"strings"
)

// executeShellCommand 执行shell命令并返回输出
// 参数：
//   - command: 要执行的shell命令
//
// 返回：
//   - string: 命令执行的输出
//   - error: 执行过程中的错误
func executeShellCommand(command string) (string, error) {
	logger.Debug("执行shell命令",
		zap.String("command", command),
	)

	cmd := exec.Command("sh", "-c", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error("命令执行失败",
			zap.Error(err),
			zap.String("output", string(output)),
		)
		return string(output), err
	}

	return string(output), nil
}

// Kubectl 执行kubectl命令并返回结果
// 支持以下特性：
// 1. 自动处理带管道的复杂命令
// 2. 支持引号和特殊字符
// 3. 智能判断命令类型并选择合适的执行方式
// 参数：
//   - command: kubectl命令（可以包含或不包含"kubectl"前缀）
//
// 返回：
//   - string: 命令执行的输出
//   - error: 执行过程中的错误
func Kubectl(command string) (string, error) {
	logger.Debug("准备执行 kubectl 命令",
		zap.String("raw_command", command),
	)

	// 移除开头的 kubectl 前缀（如果存在）
	if strings.HasPrefix(command, "kubectl") {
		command = strings.TrimSpace(strings.TrimPrefix(command, "kubectl"))
	}

	// 检查命令是否包含shell特殊字符
	// 如果包含，需要使用shell执行以支持这些特性
	if strings.Contains(command, "|") ||
		strings.Contains(command, ">") ||
		strings.Contains(command, "<") ||
		strings.Contains(command, ";") ||
		strings.Contains(command, "&&") ||
		strings.Contains(command, "||") {
		logger.Debug("检测到shell特殊字符，使用shell执行",
			zap.String("command", "kubectl "+command),
		)
		return executeShellCommand("kubectl " + command)
	}

	// 检查命令是否包含引号（通常表示复杂参数）
	if strings.Contains(command, "\"") || strings.Contains(command, "'") {
		logger.Debug("检测到引号，使用shell执行",
			zap.String("command", "kubectl "+command),
		)
		return executeShellCommand("kubectl " + command)
	}

	// 对于简单命令，使用exec.Command直接执行
	// 这种方式更安全，且避免了shell解释的开销
	logger.Debug("使用exec.Command执行简单命令",
		zap.String("command", command),
	)

	// 使用Fields分割命令，可以正确处理多个空格
	args := strings.Fields(command)
	cmd := exec.Command("kubectl", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error("kubectl 命令执行失败",
			zap.Error(err),
			zap.String("output", string(output)),
		)
		return string(output), err
	}

	logger.Debug("kubectl 命令执行成功")
	return string(output), nil
}
