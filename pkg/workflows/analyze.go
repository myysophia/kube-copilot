package workflows

import (
	"context"
	"time"

	"github.com/feiskyer/kube-copilot/pkg/utils"
	"go.uber.org/zap"

	"github.com/feiskyer/swarm-go"
)

const analysisPrompt = `As an expert on Kubernetes, your task is analyzing the given Kubernetes manifests, figure out the issues and provide solutions in a human-readable format.
For each identified issue, document the analysis and solution in everyday language, employing simple analogies to clarify technical points.

# Steps

1. **Identify Clues**: Treat each piece of YAML configuration data like a clue in a mystery. Explain how it helps to understand the issue, similar to a detective piecing together a case.
2. **Analysis with Analogies**: Translate your technical findings into relatable scenarios. Use everyday analogies to explain concepts, avoiding complex jargon. This makes episodes like 'pod failures' or 'service disruptions' simple to grasp.
3. **Solution as a DIY Guide**: Offer a step-by-step solution akin to guiding someone through a household fix-up. Instructions should be straightforward, logical, and accessible.
4. **Document Findings**:
   - Separate analysis and solution clearly for each issue, detailing them in non-technical language.

# Output Format

Provide the output in structured markdown, using clear and concise language.

# Examples

## 1. <title of the issue or potential problem>

- **Findings**: The YAML configuration doesn't specify the memory limit for the pod.
- **How to resolve**: Set memory limit in Pod spec.

## 2. HIGH Severity: CVE-2024-10963

- **Findings**: The Pod is running with CVE pam: Improper Hostname Interpretation in pam_access Leads to Access Control Bypass.
- **How to resolve**: Update package libpam-modules to fixed version (>=1.5.3) in the image. (leave the version number to empty if you don't know it)

# Notes

- Keep your language concise and simple.
- Ensure key points are included, e.g. CVE number, error code, versions.
- Relatable analogies should help in visualizing the problem and solution.
- Ensure explanations are self-contained, enough for newcomers without previous technical exposure to understand.
`

// AnalysisFlow runs a workflow to analyze Kubernetes issues and provide solutions in a human-readable format.
func AnalysisFlow(model string, manifest string, verbose bool) (string, error) {
	// 获取日志记录器
	logger := utils.GetLogger()

	// 获取性能统计工具
	perfStats := utils.GetPerfStats()
	// 开始整体工作流计时
	defer perfStats.TraceFunc("workflow_analysis_total")()

	// 记录开始时间
	startTime := time.Now()

	logger.Debug("开始执行 AnalysisFlow",
		zap.String("model", model),
		zap.Bool("verbose", verbose),
	)

	// 开始工作流初始化计时
	perfStats.StartTimer("workflow_analysis_init")

	analysisWorkflow := &swarm.Workflow{
		Name:     "analysis-workflow",
		Model:    model,
		MaxTurns: 30,
		Verbose:  verbose,
		System:   "You are an expert on Kubernetes helping user to analyze issues and provide solutions.",
		Steps: []swarm.WorkflowStep{
			{
				Name:         "analyze",
				Instructions: analysisPrompt,
				Inputs: map[string]interface{}{
					"k8s_manifest": manifest,
				},
				Functions: []swarm.AgentFunction{kubectlFunc},
			},
		},
	}

	// Create OpenAI client
	client, err := NewSwarm()

	// 停止工作流初始化计时
	initDuration := perfStats.StopTimer("workflow_analysis_init")
	logger.Debug("工作流初始化完成",
		zap.Duration("duration", initDuration),
	)

	if err != nil {
		logger.Error("创建Swarm客户端失败",
			zap.Error(err),
		)
		// 记录失败的客户端创建性能
		perfStats.RecordMetric("workflow_analysis_client_failed", initDuration)
		logger.Fatal("客户端创建失败",
			zap.Error(err),
		)
	}

	// 开始工作流执行计时
	perfStats.StartTimer("workflow_analysis_run")

	// Initialize and run workflow
	analysisWorkflow.Initialize()
	result, _, err := analysisWorkflow.Run(context.Background(), client)

	// 停止工作流执行计时
	runDuration := perfStats.StopTimer("workflow_analysis_run")

	// 记录总执行时间
	totalDuration := time.Since(startTime)

	if err != nil {
		logger.Error("工作流执行失败",
			zap.Error(err),
			zap.Duration("run_duration", runDuration),
			zap.Duration("total_duration", totalDuration),
		)
		// 记录失败的工作流执行性能
		perfStats.RecordMetric("workflow_analysis_run_failed", runDuration)
		return "", err
	}

	logger.Info("工作流执行成功",
		zap.Duration("run_duration", runDuration),
		zap.Duration("total_duration", totalDuration),
	)

	// 记录成功的工作流执行性能
	perfStats.RecordMetric("workflow_analysis_run_success", runDuration)
	// 记录模型类型的性能指标
	perfStats.RecordMetric("workflow_analysis_model_"+model, runDuration)

	return result, nil
}
