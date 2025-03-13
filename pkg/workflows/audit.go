package workflows

import (
	"context"
	"time"

	"github.com/feiskyer/kube-copilot/pkg/utils"
	"go.uber.org/zap"

	"github.com/feiskyer/swarm-go"
)

const auditPrompt = `Conduct a structured security audit of a Kubernetes environment using a Chain of Thought (CoT) approach, ensuring each technical step is clearly connected to solutions with easy-to-understand explanations.

## Plan of Action

**1. Security Auditing:**
   - **Retrieve Pod Configuration:**
      - Use "kubectl get -n {namespace} pod {pod} -o yaml" to obtain pod YAML configuration.
      - **Explain YAML:**
        - Breakdown what YAML is and its importance in understanding a pod's security posture, using analogies for clarity.

   - **Analyze YAML for Misconfigurations:**
      - Look for common security misconfigurations or risky settings within the YAML.
      - Connect issues to relatable concepts for non-technical users (e.g., likening insecure settings to an unlocked door).

**2. Vulnerability Scanning:**
   - **Extract and Scan Image:**
      - Extract the container image from the YAML configuration obtained during last step.
      - Perform a scan using "trivy image <image>".
      - Summerize Vulnerability Scans results with CVE numbers, severity, and descriptions.

**3. Issue Identification and Solution Formulation:**
   - Document each issue clearly and concisely.
   - Provide the recommendations to fix each issue.

## Provide the output in structured markdown, using clear and concise language.

Example output:

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

// AuditFlow 执行 Kubernetes Pod 的安全审计工作流
// 包括以下步骤：
// 1. 获取 Pod 配置并分析安全风险
// 2. 扫描容器镜像漏洞
// 3. 生成安全报告和修复建议
func AuditFlow(model string, namespace string, name string, verbose bool) (string, error) {
	// 获取日志记录器
	logger := utils.GetLogger()

	// 获取性能统计工具
	perfStats := utils.GetPerfStats()
	// 开始整体工作流计时
	defer perfStats.TraceFunc("workflow_audit_total")()

	// 记录开始时间
	startTime := time.Now()

	logger.Debug("开始执行 AuditFlow",
		zap.String("model", model),
		zap.String("namespace", namespace),
		zap.String("name", name),
		zap.Bool("verbose", verbose),
	)

	// 开始工作流初始化计时
	perfStats.StartTimer("workflow_audit_init")

	// 创建审计工作流
	auditWorkflow := &swarm.Workflow{
		Name:     "audit-workflow",
		Model:    model,
		MaxTurns: 30,
		Verbose:  verbose,
		System:   "You are an expert on Kubernetes helping user to audit the security issues for a given Pod.",
		Steps: []swarm.WorkflowStep{
			{
				Name:         "audit",
				Instructions: auditPrompt,
				Inputs: map[string]interface{}{
					"pod_namespace": namespace,
					"pod_name":      name,
				},
				Functions: []swarm.AgentFunction{trivyFunc, kubectlFunc},
			},
		},
	}

	// Create OpenAI client
	client, err := NewSwarm()

	// 停止工作流初始化计时
	initDuration := perfStats.StopTimer("workflow_audit_init")
	logger.Debug("工作流初始化完成",
		zap.Duration("duration", initDuration),
	)

	if err != nil {
		logger.Error("创建Swarm客户端失败",
			zap.Error(err),
		)
		// 记录失败的客户端创建性能
		perfStats.RecordMetric("workflow_audit_client_failed", initDuration)
		logger.Fatal("客户端创建失败",
			zap.Error(err),
		)
	}

	// 开始工作流执行计时
	perfStats.StartTimer("workflow_audit_run")

	// Initialize and run workflow
	auditWorkflow.Initialize()
	result, _, err := auditWorkflow.Run(context.Background(), client)

	// 停止工作流执行计时
	runDuration := perfStats.StopTimer("workflow_audit_run")

	// 记录总执行时间
	totalDuration := time.Since(startTime)

	if err != nil {
		logger.Error("工作流执行失败",
			zap.Error(err),
			zap.Duration("run_duration", runDuration),
			zap.Duration("total_duration", totalDuration),
		)
		// 记录失败的工作流执行性能
		perfStats.RecordMetric("workflow_audit_run_failed", runDuration)
		return "", err
	}

	logger.Info("工作流执行成功",
		zap.Duration("run_duration", runDuration),
		zap.Duration("total_duration", totalDuration),
	)

	// 记录成功的工作流执行性能
	perfStats.RecordMetric("workflow_audit_run_success", runDuration)
	// 记录模型类型的性能指标
	perfStats.RecordMetric("workflow_audit_model_"+model, runDuration)

	return result, nil
}
