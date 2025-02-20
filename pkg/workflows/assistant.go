package workflows

import (
	"context"
	"fmt"
	"os"

	"github.com/feiskyer/swarm-go"
)

const assistantPrompt = `As a Kubernetes expert, guide the user according to the given instructions to solve their problem or achieve their objective.

Understand the nature of their request, clarify any complex concepts, and provide step-by-step guidance tailored to their specific needs. Ensure that your explanations are comprehensive, using precise Kubernetes terminology and concepts.

# Steps

1. **Interpret User Intent**: Carefully analyze the user's instructions or questions to understand their goal.
2. **Concepts Explanation**: If necessary, break down complex Kubernetes concepts into simpler terms.
3. **Step-by-Step Solution**: Provide a detailed, clear step-by-step process to achieve the desired outcome.
4. **Troubleshooting**: Suggest potential solutions for common issues and pitfalls when working with Kubernetes.
5. **Best Practices**: Mention any relevant Kubernetes best practices that should be followed.

# Output Format

Provide a concise Markdown response in a clear, logical order. Each step should be concise, using bullet points or numbered lists if necessary. Include code snippets in markdown code blocks where relevant.

# Notes

- Assume the user has basic knowledge of Kubernetes.
- Use precise terminology and include explanations only as needed based on the complexity of the task.
- Ensure instructions are applicable across major cloud providers (GKE, EKS, AKS) unless specified otherwise.`

const assistantPrompt_cn = `翻译为中文：

作为Kubernetes专家，根据给定的指示指导用户解决问题或实现他们的目标。

理解他们请求的本质，澄清任何复杂的概念，并提供针对其特定需求量身定制的逐步指南。确保您的解释是全面的，使用精确的Kubernetes术语和概念。

# 步骤

1. **解读用户意图**：仔细分析用户的指令或问题以了解他们的目标。
2. **概念解释**：如有必要，将复杂的Kubernetes概念分解成更简单的术语。
3. **分步解决方案**：提供一个详细、清晰的分步过程来达到预期的结果。
4. **故障排除**：建议在使用Kubernetes时可能出现的问题和陷阱的潜在解决方案。
5. **最佳实践**：提及应遵循的相关Kubernetes最佳实践。

# 输出格式

提供一个简洁的Markdown响应，按清晰、逻辑顺序排列。每个步骤应该简明扼要，如果需要可以使用项目符号或编号列表。在相关的地方包括代码片段（用markdown代码块）。

# 注意事项

- 假设用户具有基本的Kubernetes知识。
- 使用精确的术语，并仅根据任务的复杂性需要进行解释。
- 除非另有说明，否则确保指示适用于主要云提供商（GKE、EKS、AKS）。`

// AssistantFlow runs a simple workflow by following the given instructions.
func AssistantFlow(model string, instructions string, verbose bool) (string, error) {
	assistantFlow := &swarm.Workflow{
		Name:     "assistant-workflow",
		Model:    model,
		MaxTurns: 30,
		Verbose:  verbose,
		System:   "You are an expert on Kubernetes helping user for the given instructions.",
		Steps: []swarm.WorkflowStep{
			{
				Name:         "assistant",
				Instructions: assistantPrompt,
				Inputs: map[string]interface{}{
					"instructions": instructions,
				},
			},
		},
	}

	// Create OpenAI client
	client, err := NewSwarm()
	if err != nil {
		fmt.Printf("Failed to create client: %v\n", err)
		os.Exit(1)
	}

	// Initialize and run workflow
	assistantFlow.Initialize()
	result, _, err := assistantFlow.Run(context.Background(), client)
	if err != nil {
		return "", err
	}

	return result, nil
}
