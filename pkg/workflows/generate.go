package workflows

import (
	"context"
	"github.com/feiskyer/kube-copilot/pkg/utils"
	"go.uber.org/zap"
	"time"

	"github.com/feiskyer/swarm-go"
)

const generatePrompt = `As a skilled technical specialist in Kubernetes and cloud-native technologies, your task is to create Kubernetes YAML manifests by following these detailed steps:

1. Review the instructions provided to generate Kubernetes YAML manifests. Ensure that these manifests adhere to current security protocols and best practices. If an instruction lacks a specific image, choose the most commonly used one from reputable sources.
2. Utilize your expertise to scrutinize the YAML manifests. Conduct a thorough step-by-step analysis to identify any issues. Resolve these issues, ensuring the YAML manifests are accurate and secure.
3. After fixing and verifying the manifests, compile them in their raw form. For multiple YAML files, use '---' as a separator.

# Steps

1. **Understand the Instructions:**
   - Evaluate the intended use and environment for each manifest as per instructions provided.

2. **Security and Best Practices Assessment:**
   - Assess the security aspects of each component, ensuring alignment with current standards and best practices.
   - Perform a comprehensive analysis of the YAML structure and configurations.

3. **Document and Address Discrepancies:**
   - Document and justify any discrepancies or issues you find, in a sequential manner.
   - Implement robust solutions that enhance the manifests' performance and security, utilizing best practices and recommended images.

4. **Finalize the YAML Manifests:**
   - Ensure the final manifests are syntactically correct, properly formatted, and deployment-ready.

# Output Format

- Present only the final YAML manifests in raw format, separated by "---" for multiple files.
- Exclude any comments or additional annotations within the YAML files.

Your expertise ensures these manifests are not only functional but also compliant with the highest standards in Kubernetes and cloud-native technologies.`

// GeneratorFlow runs a workflow to generate Kubernetes YAML manifests based on the provided instructions.
func GeneratorFlow(model string, instructions string, verbose bool) (string, error) {
	// 获取日志记录器
	logger := utils.GetLogger()

	// 获取性能统计工具
	perfStats := utils.GetPerfStats()
	// 开始整体工作流计时
	defer perfStats.TraceFunc("workflow_generator_total")()

	// 记录开始时间
	startTime := time.Now()

	logger.Debug("开始执行 GeneratorFlow",
		zap.String("model", model),
		zap.String("instructions", instructions),
		zap.Bool("verbose", verbose),
	)

	// 开始工作流初始化计时
	perfStats.StartTimer("workflow_generator_init")

	generatorWorkflow := &swarm.Workflow{
		Name:     "generator-workflow",
		Model:    model,
		MaxTurns: 30,
		Verbose:  verbose,
		System:   "You are an expert on Kubernetes helping user to generate Kubernetes YAML manifests.",
		Steps: []swarm.WorkflowStep{
			{
				Name:         "generator",
				Instructions: generatePrompt,
				Inputs: map[string]interface{}{
					"instructions": instructions,
				},
			},
		},
	}

	// Create OpenAI client
	client, err := NewSwarm()

	// 停止工作流初始化计时
	initDuration := perfStats.StopTimer("workflow_generator_init")
	logger.Debug("工作流初始化完成",
		zap.Duration("duration", initDuration),
	)

	if err != nil {
		logger.Error("创建Swarm客户端失败",
			zap.Error(err),
		)
		// 记录失败的客户端创建性能
		perfStats.RecordMetric("workflow_generator_client_failed", initDuration)
		logger.Fatal("客户端创建失败",
			zap.Error(err),
		)
	}

	// 开始工作流执行计时
	perfStats.StartTimer("workflow_generator_run")

	// Initialize and run workflow
	generatorWorkflow.Initialize()
	result, _, err := generatorWorkflow.Run(context.Background(), client)

	// 停止工作流执行计时
	runDuration := perfStats.StopTimer("workflow_generator_run")

	// 记录总执行时间
	totalDuration := time.Since(startTime)

	if err != nil {
		logger.Error("工作流执行失败",
			zap.Error(err),
			zap.Duration("run_duration", runDuration),
			zap.Duration("total_duration", totalDuration),
		)
		// 记录失败的工作流执行性能
		perfStats.RecordMetric("workflow_generator_run_failed", runDuration)
		return "", err
	}

	logger.Info("工作流执行成功",
		zap.Duration("run_duration", runDuration),
		zap.Duration("total_duration", totalDuration),
	)

	// 记录成功的工作流执行性能
	perfStats.RecordMetric("workflow_generator_run_success", runDuration)
	// 记录模型类型的性能指标
	perfStats.RecordMetric("workflow_generator_model_"+model, runDuration)

	return result, nil
}
