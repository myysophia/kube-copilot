package workflows

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/feiskyer/kube-copilot/pkg/tools"
	"github.com/feiskyer/swarm-go"
)

var (
	// trivyFunc 是用于执行容器镜像安全扫描的函数
	trivyFunc = swarm.NewAgentFunction(
		"trivy",
		"Run trivy image scanning for a given image",
		func(args map[string]interface{}) (interface{}, error) {
			image, ok := args["image"].(string)
			if !ok {
				return nil, fmt.Errorf("image not provided")
			}

			result, err := tools.Trivy(image)
			if err != nil {
				return nil, err
			}

			return result, nil
		},
		[]swarm.Parameter{
			{Name: "image", Type: reflect.TypeOf(""), Required: true},
		},
	)

	// kubectlFunc 是用于执行 kubectl 命令的函数
	kubectlFunc = swarm.NewAgentFunction(
		"kubectl",
		"Run kubectl command",
		func(args map[string]interface{}) (interface{}, error) {
			command, ok := args["command"].(string)
			if !ok {
				return nil, fmt.Errorf("command not provided")
			}

			result, err := tools.Kubectl(command)
			if err != nil {
				return nil, err
			}

			return result, nil
		},
		[]swarm.Parameter{
			{Name: "command", Type: reflect.TypeOf(""), Required: true},
		},
	)
)

// NewSwarm 创建新的 Swarm 客户端
// 支持多种 LLM 提供商：
// - OpenAI API
// - Azure OpenAI
// - 其他 OpenAI 兼容的 LLM
func NewSwarm() (*swarm.Swarm, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY is not set")
	}

	baseURL := os.Getenv("OPENAI_API_BASE")
	// OpenAI
	if baseURL == "" {
		return swarm.NewSwarm(swarm.NewOpenAIClient(apiKey)), nil
	}

	// Azure OpenAI
	if strings.Contains(baseURL, "azure") {
		return swarm.NewSwarm(swarm.NewAzureOpenAIClient(apiKey, baseURL)), nil
	}

	// OpenAI compatible LLM
	return swarm.NewSwarm(swarm.NewOpenAIClientWithBaseURL(apiKey, baseURL)), nil
}
