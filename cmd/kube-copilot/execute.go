/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package main

import (
	"fmt"
	"strings"

	//"github.com/fatih/color"
	"github.com/feiskyer/kube-copilot/pkg/assistants"
	"github.com/feiskyer/kube-copilot/pkg/tools"
	kubetools "github.com/feiskyer/kube-copilot/pkg/tools"
	"github.com/feiskyer/kube-copilot/pkg/utils"
	"github.com/feiskyer/kube-copilot/pkg/workflows"
	"github.com/sashabaranov/go-openai"
	"github.com/spf13/cobra"
	//"github.com/spf13/pflag"
	//"github.com/spf13/viper"
	"go.uber.org/zap"
)

const executeSystemPrompt = `As a technical expert in Kubernetes and cloud-native networking, your task follows a specific Chain of Thought methodology to ensure thoroughness and accuracy while adhering to the constraints provided.
Available Tools:
- kubectl: Useful for executing kubectl commands. Remember to use '--sort-by=memory' or '--sort-by=cpu' when running 'kubectl top' command.  Input: a kubectl command. Output: the result of the command.
- python: This is a Python interpreter. Use it for executing Python code with the Kubernetes Python SDK client. Ensure the results are output using "print(...)". The input is a Python script, and the output will be the stdout and stderr of this script.
- trivy: Useful for executing trivy image command to scan images for vulnerabilities. Input: an image for security scanning. Output: the vulnerabilities found in the image.

The steps you take are as follows:

1. Problem Identification: Begin by clearly defining the problem you're addressing. When diagnostics or troubleshooting is needed, specify the symptoms or issues observed that prompted the analysis. This helps to narrow down the potential causes and guides the subsequent steps.
2. Diagnostic Commands: Utilize 'python' tool to gather information about the state of the Kubernetes resources, network policies, and other related configurations. Detail why each command is chosen and what information it is expected to yield. In cases where 'trivy' is applicable, explain how it will be used to analyze container images for vulnerabilities.
3. Interpretation of Outputs: Analyze the outputs from the executed commands. Describe what the results indicate about the health and configuration of the system and network. This is crucial for identifying any discrepancies that may be contributing to the issue at hand.
4. Troubleshooting Strategy: Based on the interpreted outputs, develop a step-by-step strategy for troubleshooting. Justify each step within the strategy, explaining how it relates to the findings from the diagnostic outputs.
5. Actionable Solutions: Propose solutions that can be carried out using 'kubectl' commands, where possible. If the solution involves a sequence of actions, explain the order and the expected outcome of each. For issues identified by 'trivy', provide recommendations for remediation based on best practices.
6. Contingency for Unavailable Tools: In the event that the necessary tools or commands are unavailable, provide an alternative set of instructions that comply with the guidelines, explaining how these can help progress the troubleshooting process.

Throughout this process, ensure that each response is concise and strictly adheres to the guidelines provided, with a clear justification for each step taken. The ultimate goal is to identify the root cause of issues within the domains of Kubernetes and cloud-native networking and to provide clear, actionable solutions, while staying within the operational constraints of 'kubectl' or 'trivy image' for diagnostics and troubleshooting and avoiding any installation operations.

Use this JSON format for responses:

{
	"question": "<input question>",
	"thought": "<your thought process>",
	"action": {
		"name": "<action to take, choose from tools [kubectl, python, trivy]. Do not set final_answer when an action is required>",
		"input": "<input for the action. ensure all contexts are added as input if required, e.g. raw YAML or image name.>"
	},
	"observation": "<result of the action, set by external tools>",
	"final_answer": "<your final findings, only set after completed all processes and no action is required>"
}
note: please always use chinese reply
`

//const executeSystemPrompt_cn = `您是Kubernetes和云原生网络的技术专家，您的任务是遵循特定的链式思维方法，以确保在遵守约束的情况下实现彻底性和准确性。
//
//可用工具：
//- kubectl：用于执行 Kubernetes 命令。输入：一个独立的 kubectl 命令（例如 'get pods -o json'），不支持直接包含管道或后续处理命令。输出：命令的结果，通常为 JSON 或文本格式。如果运行"kubectl top"，使用"--sort-by=memory"或"--sort-by=cpu"排序。
//- python：用于执行带有 Kubernetes Python SDK 的 Python 代码。输入：Python 脚本。输出：脚本的 stdout 和 stderr，使用 print(...) 输出结果。
//- trivy：用于扫描容器镜像中的漏洞。输入：镜像名称（例如 'nginx:latest'）。输出：漏洞报告。
//- jq：用于处理和查询 JSON 数据。输入：一个有效的 jq 表达式（例如 '-r .items[] | select(.metadata.name | test("iotdb")) | .spec.containers[].image'），需配合前一步的 JSON 输出使用。输出：查询结果。确保表达式针对 kubectl 返回的 JSON 结构设计。
//
//您采取的步骤如下：
//1. 问题识别：清楚定义问题，描述观察到的症状或目标。
//2. 诊断命令：优先使用 kubectl 获取相关数据（如 JSON 输出），说明命令选择理由。如果需要进一步处理，使用 jq 分析前一步的结果。若适用 trivy，解释其用于镜像漏洞分析的原因。
//3. 输出解释：分析命令输出，描述系统状态、健康状况或配置情况，识别潜在问题。
//4. 故障排除策略：根据输出制定分步策略，证明每步如何与诊断结果相关。
//5. 可行解决方案：提出可执行的解决方案，优先使用 kubectl 命令。若涉及多步操作，说明顺序和预期结果。对于 trivy 识别的漏洞，基于最佳实践提供补救建议。
//6. 应急方案：如果工具不可用或命令失败，提供替代方法（如分步执行替代管道操作），确保仍能推进故障排除。
//
//约束：
//- 优先使用 kubectl 获取数据，配合grep来过滤关键字来减少token的消耗，单步执行优先。
//- 确保每步操作在单次 action 中完成（如获取 Pod 和提取镜像版本分两步），无需用户手动干预。
//- 禁止安装操作，所有步骤在现有工具约束内完成。
//
//重要提示：您必须始终使用以下 JSON 格式返回响应。不要直接返回 Markdown 文本。所有格式化的文本都应该放在 final_answer 字段中：
//
//{
//	"question": "<输入问题>",
//	"thought": "<思维过程>",
//	"action": {
//		"name": "<工具名，从 [kubectl, python, trivy, jq] 中选择>",
//		"input": "<工具输入，确保包含所有必要上下文>"
//	},
//	"observation": "<工具执行结果，由外部填充>",
//	"final_answer": "<最终答案，使用清晰的 Markdown 格式，包含适当的标题、列表和代码块。对于执行结果，提供简洁的总结和必要的解释。使用中文回答。>"
//}
//
//目标：
//在 Kubernetes 和云原生网络领域内识别问题根本原因，提供清晰、可行的解决方案，同时保持诊断和故障排除的运营约束。`

const executeSystemPrompt_cn = `您是Kubernetes和云原生网络的技术专家，您的任务是遵循链式思维方法，确保在遵守约束的情况下实现彻底性和准确性。

可用工具：
- kubectl：用于执行 Kubernetes 命令。必须优先使用管道和过滤条件（例如 'kubectl get pods -o json | grep "pod-name"'），禁止直接运行无过滤的全量命令（如 'kubectl get pods -o json'）。如果运行 "kubectl top"，使用 "--sort-by=memory" 或 "--sort-by=cpu" 排序。
- python：用于执行带有 Kubernetes Python SDK 的 Python 代码。输入：Python 脚本。输出：脚本的 stdout 和 stderr，使用 print(...) 输出结果。
- trivy：用于扫描容器镜像中的漏洞。输入：镜像名称（例如 'nginx:latest'）。输出：漏洞报告。
- jq：用于处理和查询 JSON 数据。输入：有效的 jq 表达式（例如 'jq -r .items[] | select(.metadata.name | test("pod-name")) | .spec.containers[].image'），需配合前一步的 JSON 输出使用。

您采取的步骤如下：
1. 问题识别：清楚定义问题，描述观察到的症状或目标
2. 诊断命令：优先使用 kubectl 获取相关数据，必须包含过滤条件（如 namespace、label 或名称），说明命令选择理由。若适用 trivy，解释其用于镜像漏洞分析的原因。
3. 输出解释：分析命令输出，描述系统状态、健康状况或配置情况，识别潜在问题。
4. 故障排除策略：根据输出制定分步策略，证明每步如何与诊断结果相关。
5. 可行解决方案：提出可执行的解决方案，优先使用带过滤条件的 kubectl 命令。若涉及多步操作，说明顺序和预期结果。对于 trivy 识别的漏洞，基于最佳实践提供补救建议。

严格约束：
- 禁止获取全量数据（如 'kubectl get pods -o json' 或 'kubectl get nodes -o json'），每次命令必须使用过滤条件（例如 '-n namespace'、'grep "keyword"' 或 label 选择器）。
- 优先使用管道（如 grep）减少数据量，确保单步操作 Token 消耗最小化。
- 确保每步操作在单次 action 中完成，无需用户手动干预。

示例：
- 问题："检查 pod my-app 的状态"
- 正确命令：'kubectl get pods -o json | grep "my-app"'
- 错误命令：'kubectl get pods -o json'（全量数据，禁止使用）

重要提示：您必须始终使用以下 JSON 格式返回响应。所有格式化的文本放在 final_answer 字段中：
{
  "question": "<输入问题>",
  "thought": "<思维过程>",
  "action": {
    "name": "<工具名，从 [kubectl, python, jq, trivy] 中选择>",
    "input": "<工具输入，必须包含过滤条件>"
  },
  "observation": "<工具执行结果，由外部填充>",
  "final_answer": "<最终答案，使用清晰的 Markdown 格式，换行符用 \\n 表示，例如：'### 标题\\n- 列表项1\\n- 列表项2'>"
}

目标：在 Kubernetes 和云原生网络领域内识别问题根本原因，提供清晰、可行的解决方案，同时保持诊断和故障排除的运营约束。`

var instructions string
var model string

//var maxTokens int
//var countTokens int
//var verbose bool
//var maxIterations int
//var logger *logrus.Logger

func init() {
	tools.CopilotTools["trivy"] = kubetools.Trivy

	executeCmd.PersistentFlags().StringVarP(&instructions, "instructions", "", "", "instructions to execute")
	executeCmd.MarkFlagRequired("instructions")

	executeCmd.PersistentFlags().StringVarP(&model, "model", "", "gpt-3.5-turbo", "model to use")
	executeCmd.PersistentFlags().IntVarP(&maxTokens, "max-tokens", "", 1024, "max tokens for the model")
	//executeCmd.PersistentFlags().IntVarP(&countTokens, "count-tokens", "", 1024, "count tokens for the model")
	executeCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "", true, "verbose output")
	executeCmd.PersistentFlags().IntVarP(&maxIterations, "max-iterations", "", 10, "max iterations for the model")

	//logger = logrus.New()
}

var executeCmd = &cobra.Command{
	Use:   "execute",
	Short: "Execute operations based on prompt instructions",
	Run: func(cmd *cobra.Command, args []string) {
		// 确保日志已初始化
		if logger == nil {
			initLogger()
			defer logger.Sync()
		}

		if instructions == "" && len(args) > 0 {
			instructions = strings.Join(args, " ")
		}
		if instructions == "" {
			logger.Fatal("执行失败",
				zap.String("error", "缺少必要参数: instructions"),
			)
			return
		}

		logger.Info("开始执行指令",
			zap.String("instructions", instructions),
			zap.String("model", model),
		)

		messages := []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: executeSystemPrompt_cn,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: fmt.Sprintf("Here are the instructions: %s", instructions),
			},
		}

		logger.Debug("发送请求到 OpenAI",
			zap.Any("messages", messages),
			zap.Int("maxTokens", maxTokens),
			zap.Bool("countTokens", countTokens),
			zap.Bool("verbose", verbose),
			zap.Int("maxIterations", maxIterations),
		)

		response, _, err := assistants.Assistant(model, messages, maxTokens, countTokens, verbose, maxIterations)
		if err != nil {
			logger.Error("执行失败",
				zap.Error(err),
			)
			return
		}

		logger.Debug("收到原始响应",
			zap.String("response", response),
		)

		formatInstructions := fmt.Sprintf("Extract the execuation results for user instructions and reformat in a concise Markdown response: %s", response)
		result, err := workflows.AssistantFlow(model, formatInstructions, verbose)
		if err != nil {
			logger.Error("格式化结果失败",
				zap.Error(err),
				zap.String("raw_response", response),
			)
			return
		}

		logger.Info("执行完成",
			zap.String("result", result),
		)
		utils.RenderMarkdown(result)
	},
}
