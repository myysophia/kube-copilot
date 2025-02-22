package tools

import (
	"os/exec"
	"strings"
	"go.uber.org/zap"
)

// Kubectl runs the given kubectl command and returns the output.
func Kubectl(command string) (string, error) {
	logger.Debug("准备执行 kubectl 命令",
		zap.String("raw_command", command),
	)

	if strings.HasPrefix(command, "kubectl") {
		command = strings.TrimSpace(strings.TrimPrefix(command, "kubectl"))
	}

	cmd := exec.Command("kubectl", strings.Split(command, " ")...)
	logger.Debug("构建命令",
		zap.String("command", command),
		zap.Strings("args", strings.Split(command, " ")),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error("kubectl 命令执行失败",
			zap.Error(err),
			zap.String("output", string(output)),
		)
		return strings.TrimSpace(string(output)), err
	}

	logger.Debug("kubectl 命令执行成功",
		zap.String("output", string(output)),
	)
	return strings.TrimSpace(string(output)), nil
}
