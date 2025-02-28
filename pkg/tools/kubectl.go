package tools

import (
	"fmt"
	"go.uber.org/zap"
	"os/exec"
	"strings"
)

// executeShellCommand 执行shell命令并返回输出
// 参数：
//   - command: 要执行的shell命令
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

// 处理JSON输出，提取关键信息
func processJSONOutput(output string) (string, error) {
	// 使用jq提取关键信息
	jqCmd := `jq -c '.items[] | {name: .metadata.name, namespace: .metadata.namespace, kind: .kind, status: .status.phase}'`
	return executeShellCommand(fmt.Sprintf("echo '%s' | %s", output, jqCmd))
}

// SmartResourceQuery 智能查询Kubernetes资源
// 功能特性：
// 1. 支持模糊匹配资源名称
// 2. 自动处理命名空间
// 3. 根据结果数量返回不同详细程度的信息
// 4. 智能降级搜索（当精确匹配失败时尝试更宽泛的搜索）
// 参数：
//   - resourceName: 要查询的资源名称（支持模糊匹配）
// 返回：
//   - string: 查询结果
//   - error: 查询过程中的错误
func SmartResourceQuery(resourceName string) (string, error) {
	logger.Debug("执行智能资源查询",
		zap.String("resourceName", resourceName),
	)

	// 清理输入，移除可能的引号
	resourceName = strings.Trim(resourceName, "\"'")

	// 构建初始查询命令
	// 查找多种资源类型，支持跨命名空间
	command := fmt.Sprintf("kubectl get deployment,statefulset,daemonset,pod,service,configmap,secret -A -o name | grep -i \"%s\"", resourceName)

	// 执行查询
	result, err := executeShellCommand(command)
	if err != nil {
		// 如果精确匹配失败，尝试更宽泛的搜索
		if strings.Contains(string(result), "No resources found") || result == "" {
			logger.Debug("未找到精确匹配，尝试更宽泛搜索")
			// 使用资源名称的第一部分进行搜索
			command = fmt.Sprintf("kubectl get all -A -o name | grep -i \"%s\"",
				strings.Split(resourceName, "-")[0])
			return executeShellCommand(command)
		}
		return result, err
	}

	// 处理查询结果
	if result != "" {
		lines := strings.Split(strings.TrimSpace(result), "\n")
		if len(lines) == 1 {
			// 只找到一个资源，返回详细信息
			resourceType := strings.Split(lines[0], "/")[0]
			name := strings.Split(lines[0], "/")[1]

			// 处理命名空间信息
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

				// 处理命名空间信息
				namespace := ""
				if strings.Contains(name, ".") {
					parts := strings.Split(name, ".")
					name = parts[0]
					namespace = parts[1]
				}

				// 获取资源的基本信息
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
