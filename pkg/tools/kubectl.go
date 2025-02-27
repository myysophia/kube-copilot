package tools

import (
	"fmt"
	"go.uber.org/zap"
	"os/exec"
	"strings"
)

// 执行shell命令（支持管道）
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

// Kubectl runs the given kubectl command and returns the output.
func Kubectl(command string) (string, error) {
	logger.Debug("准备执行 kubectl 命令",
		zap.String("raw_command", command),
	)

	// 移除开头的 kubectl
	if strings.HasPrefix(command, "kubectl") {
		command = strings.TrimSpace(strings.TrimPrefix(command, "kubectl"))
	}

	// 检查命令是否包含管道操作符或其他shell特殊字符
	if strings.Contains(command, "|") ||
		strings.Contains(command, ">") ||
		strings.Contains(command, "<") ||
		strings.Contains(command, ";") ||
		strings.Contains(command, "&&") ||
		strings.Contains(command, "||") {
		// 使用shell执行完整命令
		logger.Debug("检测到shell特殊字符，使用shell执行",
			zap.String("command", "kubectl "+command),
		)
		return executeShellCommand("kubectl " + command)
	}

	// 检查命令是否包含引号，这可能表示复杂参数
	if strings.Contains(command, "\"") || strings.Contains(command, "'") {
		logger.Debug("检测到引号，使用shell执行",
			zap.String("command", "kubectl "+command),
		)
		return executeShellCommand("kubectl " + command)
	}

	// 对于简单命令，使用exec.Command直接执行
	logger.Debug("使用exec.Command执行简单命令",
		zap.String("command", command),
	)

	// 使用Fields而不是Split来正确处理多个空格
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

// 处理JSON输出，提取关键信息
func processJSONOutput(output string) (string, error) {
	// 使用jq提取关键信息
	jqCmd := `jq -c '.items[] | {name: .metadata.name, namespace: .metadata.namespace, kind: .kind, status: .status.phase}'`
	return executeShellCommand(fmt.Sprintf("echo '%s' | %s", output, jqCmd))
}

// 智能资源查询函数
func SmartResourceQuery(resourceName string) (string, error) {
	logger.Debug("执行智能资源查询",
		zap.String("resourceName", resourceName),
	)

	// 移除可能的引号
	resourceName = strings.Trim(resourceName, "\"'")

	// 构建查询命令 - 先查找所有可能的资源类型
	command := fmt.Sprintf("kubectl get deployment,statefulset,daemonset,pod,service,configmap,secret -A -o name | grep -i \"%s\"", resourceName)

	// 执行查询
	result, err := executeShellCommand(command)
	if err != nil {
		// 如果没有找到匹配项，尝试更宽泛的搜索
		if strings.Contains(string(result), "No resources found") || result == "" {
			logger.Debug("未找到精确匹配，尝试更宽泛搜索")
			// 尝试使用部分名称匹配
			command = fmt.Sprintf("kubectl get all -A -o name | grep -i \"%s\"",
				strings.Split(resourceName, "-")[0]) // 使用第一个连字符前的部分
			return executeShellCommand(command)
		}
		return result, err
	}

	// 如果找到了资源，获取更详细的信息
	if result != "" {
		lines := strings.Split(strings.TrimSpace(result), "\n")
		if len(lines) == 1 {
			// 只找到一个资源，获取详细信息
			resourceType := strings.Split(lines[0], "/")[0]
			name := strings.Split(lines[0], "/")[1]

			// 提取命名空间（如果有）
			namespace := ""
			if strings.Contains(lines[0], ".") {
				parts := strings.Split(name, ".")
				name = parts[0]
				namespace = parts[1]
			}

			// 构建详细查询命令
			detailCommand := fmt.Sprintf("kubectl get %s %s", resourceType, name)
			if namespace != "" {
				detailCommand += " -n " + namespace
			}
			detailCommand += " -o yaml"

			return executeShellCommand(detailCommand)
		} else if len(lines) > 1 && len(lines) <= 5 {
			// 找到多个资源但数量适中，获取所有资源的基本信息
			var detailResults []string

			for _, line := range lines {
				if line == "" {
					continue
				}

				resourceType := strings.Split(line, "/")[0]
				name := strings.Split(line, "/")[1]

				// 提取命名空间（如果有）
				namespace := ""
				if strings.Contains(name, ".") {
					parts := strings.Split(name, ".")
					name = parts[0]
					namespace = parts[1]
				}

				// 构建详细查询命令
				detailCommand := fmt.Sprintf("kubectl get %s %s", resourceType, name)
				if namespace != "" {
					detailCommand += " -n " + namespace
				}
				detailCommand += " -o wide"

				detail, _ := executeShellCommand(detailCommand)
				detailResults = append(detailResults, detail)
			}

			return strings.Join(detailResults, "\n\n"), nil
		} else {
			// 找到太多资源，返回摘要信息
			return fmt.Sprintf("找到 %d 个匹配资源:\n%s\n\n请提供更具体的资源名称以获取详细信息。",
				len(lines), result), nil
		}
	}

	return result, nil
}
