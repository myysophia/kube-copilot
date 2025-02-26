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
package assistants

import (
	"encoding/json"
	"fmt"
	"github.com/fatih/color"
	"github.com/feiskyer/kube-copilot/pkg/llms"
	"github.com/feiskyer/kube-copilot/pkg/tools"
	"github.com/sashabaranov/go-openai"
	"go.uber.org/zap"
	"regexp"
	"strings"
)

var logger *zap.Logger

func init() {
	var err error
	logger, err = zap.NewProduction()
	if err != nil {
		panic(fmt.Sprintf("无法初始化日志: %v", err))
	}
}

const (
	defaultMaxIterations = 10
)

// Assistant is the simplest AI assistant.
func Assistant(model string, prompts []openai.ChatCompletionMessage, maxTokens int, countTokens bool, verbose bool, maxIterations int) (result string, chatHistory []openai.ChatCompletionMessage, err error) {
	logger.Info("开始执行 Assistant",
		zap.String("model", model),
		zap.Int("maxTokens", maxTokens),
		zap.Bool("countTokens", countTokens),
		zap.Bool("verbose", verbose),
		zap.Int("maxIterations", maxIterations),
	)

	chatHistory = prompts
	if len(prompts) == 0 {
		logger.Error("提示信息为空")
		return "", nil, fmt.Errorf("prompts cannot be empty")
	}

	client, err := llms.NewOpenAIClient("", "")
	if err != nil {
		logger.Error("创建 OpenAI 客户端失败",
			zap.Error(err),
		)
		return "", nil, fmt.Errorf("unable to get OpenAI client: %v", err)
	}

	defer func() {
		if countTokens {
			count := llms.NumTokensFromMessages(chatHistory, model)
			logger.Info("Token 统计",
				zap.Int("total_tokens", count),
			)
			color.Green("Total tokens: %d\n\n", count)
		}
	}()

	if verbose {
		logger.Debug("开始第一轮对话")
		color.Blue("Iteration 1): chatting with LLM\n")
	}

	resp, err := client.Chat(model, maxTokens, chatHistory)
	cleanedResp := cleanJSON(resp)
	logger.Debug("清理后的响应",
		zap.String("response", cleanedResp),
	)
	if err != nil {
		logger.Error("对话完成失败",
			zap.Error(err),
		)
		return "", chatHistory, fmt.Errorf("chat completion error: %v", err)
	}

	chatHistory = append(chatHistory, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleAssistant,
		Content: string(resp),
	})

	if verbose {
		logger.Debug("LLM 初始响应",
			zap.String("response", resp),
		)
		color.Cyan("Initial response from LLM:\n%s\n\n", resp)
	}

	var toolPrompt tools.ToolPrompt
	if err = json.Unmarshal([]byte(cleanedResp), &toolPrompt); err != nil {
		if verbose {
			logger.Warn("无法解析工具提示，假定为最终答案",
				zap.Error(err),
				zap.String("response", resp),
			)
			color.Cyan("Unable to parse tool from prompt, assuming got final answer.\n\n", resp)
			color.Cyan("json marshal error: %s\n\n", err)
		}
		return cleanedResp, chatHistory, nil
	}

	iterations := 0
	if maxIterations <= 0 {
		maxIterations = defaultMaxIterations
	}
	for {
		iterations++
		logger.Debug("开始新的迭代",
			zap.Int("iteration", iterations),
			zap.String("thought", toolPrompt.Thought),
		)

		if verbose {
			color.Cyan("Thought: %s\n\n", toolPrompt.Thought)
		}

		if iterations > maxIterations {
			logger.Warn("达到最大迭代次数",
				zap.Int("maxIterations", maxIterations),
			)
			color.Red("Max iterations reached")
			return toolPrompt.FinalAnswer, chatHistory, nil
		}

		if toolPrompt.FinalAnswer != "" {
			logger.Info("获得最终答案",
				zap.String("finalAnswer", toolPrompt.FinalAnswer),
			)
			if verbose {
				color.Cyan("Final answer: %v\n\n", toolPrompt.FinalAnswer)
			}
			return toolPrompt.FinalAnswer, chatHistory, nil
		}

		if toolPrompt.Action.Name != "" {
			var observation string
			logger.Debug("执行工具",
				zap.String("tool", toolPrompt.Action.Name),
				zap.String("input", toolPrompt.Action.Input),
			)

			if verbose {
				color.Blue("Iteration %d): executing tool %s\n", iterations, toolPrompt.Action.Name)
				color.Cyan("Invoking %s tool with inputs: \n============\n%s\n============\n\n", toolPrompt.Action.Name, toolPrompt.Action.Input)
			}

			if toolFunc, ok := tools.CopilotTools[toolPrompt.Action.Name]; ok {
				ret, err := toolFunc(toolPrompt.Action.Input)
				observation = strings.TrimSpace(ret)
				if err != nil {
					logger.Error("工具执行失败",
						zap.String("tool", toolPrompt.Action.Name),
						zap.Error(err),
					)
					observation = fmt.Sprintf("Tool %s failed with error %s. Considering refine the inputs for the tool.", toolPrompt.Action.Name, ret)
				} else {
					logger.Debug("工具执行成功",
						zap.String("tool", toolPrompt.Action.Name),
						zap.String("observation", observation),
					)
				}
			} else {
				logger.Warn("工具不可用",
					zap.String("tool", toolPrompt.Action.Name),
				)
				observation = fmt.Sprintf("Tool %s is not available. Considering switch to other supported tools.", toolPrompt.Action.Name)
			}

			if verbose {
				color.Cyan("Observation: %s\n\n", observation)
			}

			// Constrict the prompt to the max tokens allowed by the model.
			// This is required because the tool may have generated a long output.
			observation = llms.ConstrictPrompt(observation, model, 1024)
			toolPrompt.Observation = observation
			assistantMessage, _ := json.Marshal(toolPrompt)
			chatHistory = append(chatHistory, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleUser,
				Content: string(assistantMessage),
			})
			// Constrict the chat history to the max tokens allowed by the model.
			// This is required because the chat history may have grown too large.
			chatHistory = llms.ConstrictMessages(chatHistory, model, maxTokens)

			// Start next iteration of LLM chat.
			if verbose {
				color.Blue("Iteration %d): chatting with LLM\n", iterations)
			}

			resp, err := client.Chat(model, maxTokens, chatHistory)
			if err != nil {
				logger.Error("对话完成失败",
					zap.Error(err),
				)
				return "", chatHistory, fmt.Errorf("chat completion error: %v", err)
			}

			chatHistory = append(chatHistory, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleAssistant,
				Content: string(resp),
			})
			if verbose {
				logger.Debug("LLM 中间响应",
					zap.String("response", resp),
				)
				color.Cyan("Intermediate response from LLM: %s\n\n", resp)
			}

			// extract the tool prompt from the LLM response.
			if err = json.Unmarshal([]byte(resp), &toolPrompt); err != nil {
				if verbose {
					logger.Warn("无法从 LLM 解析工具，总结最终答案",
						zap.Error(err),
					)
					color.Cyan("Unable to parse tools from LLM (%s), summarizing the final answer.\n\n", err.Error())
				}

				chatHistory = append(chatHistory, openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleUser,
					Content: "Summarize all the chat history and respond to original question with final answer",
				})

				resp, err = client.Chat(model, maxTokens, chatHistory)
				if err != nil {
					logger.Error("总结对话失败",
						zap.Error(err),
					)
					return "", chatHistory, fmt.Errorf("chat completion error: %v", err)
				}

				logger.Info("完成总结",
					zap.String("summary", resp),
				)
				
				// 尝试从响应中提取final_answer
				var finalResponse map[string]interface{}
				if err := json.Unmarshal([]byte(resp), &finalResponse); err == nil {
					if finalAnswer, ok := finalResponse["final_answer"].(string); ok && finalAnswer != "" {
						logger.Info("成功提取final_answer",
							zap.String("final_answer", finalAnswer),
						)
						return finalAnswer, chatHistory, nil
					}
				}
				
				return resp, chatHistory, nil
			}
		}
	}
}

// AssistantWithConfig 是支持自定义配置的 AI 助手
func AssistantWithConfig(model string, prompts []openai.ChatCompletionMessage, maxTokens int, countTokens bool, verbose bool, maxIterations int, apiKey string, baseUrl string) (result string, chatHistory []openai.ChatCompletionMessage, err error) {
	logger.Info("开始执行 AssistantWithConfig",
		zap.String("model", model),
		zap.Int("maxTokens", maxTokens),
		zap.Bool("countTokens", countTokens),
		zap.Bool("verbose", verbose),
		zap.Int("maxIterations", maxIterations),
		zap.String("baseUrl", baseUrl),
	)

	chatHistory = prompts
	if len(prompts) == 0 {
		logger.Error("提示信息为空")
		return "", nil, fmt.Errorf("prompts cannot be empty")
	}

	// 使用自定义配置创建客户端
	config := openai.DefaultConfig(apiKey)
	if baseUrl != "" {
		config.BaseURL = baseUrl
		if strings.Contains(baseUrl, "azure") {
			config.APIType = openai.APITypeAzure
			config.APIVersion = "2024-06-01"
			config.AzureModelMapperFunc = func(model string) string {
				return regexp.MustCompile(`[.:]`).ReplaceAllString(model, "")
			}
		}
	}
	client, err := llms.NewOpenAIClient(apiKey, baseUrl)
	if err != nil {
		logger.Error("创建 OpenAI 客户端失败",
			zap.Error(err),
		)
		return "", nil, fmt.Errorf("unable to get OpenAI client: %v", err)
	}
	//client := &OpenAIClient{
	//	Retries: 5,
	//	Backoff: time.Second,
	//	Client:  openai.NewClientWithConfig(config),
	//}

	defer func() {
		if countTokens {
			count := llms.NumTokensFromMessages(chatHistory, model)
			logger.Info("Token 统计",
				zap.Int("total_tokens", count),
			)
			color.Green("Total tokens: %d\n\n", count)
		}
	}()

	if verbose {
		logger.Debug("开始第一轮对话")
		color.Blue("Iteration 1): chatting with LLM\n")
	}

	resp, err := client.Chat(model, maxTokens, chatHistory)
	cleanedResp := cleanJSON(resp)
	logger.Debug("清理后的响应",
		zap.String("response", cleanedResp),
	)
	if err != nil {
		logger.Error("对话完成失败",
			zap.Error(err),
		)
		return "", chatHistory, fmt.Errorf("chat completion error: %v", err)
	}

	chatHistory = append(chatHistory, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleAssistant,
		Content: string(resp),
	})

	if verbose {
		logger.Debug("LLM 初始响应",
			zap.String("response", resp),
		)
		color.Cyan("Initial response from LLM:\n%s\n\n", resp)
	}

	var toolPrompt tools.ToolPrompt
	if err = json.Unmarshal([]byte(cleanedResp), &toolPrompt); err != nil {
		if verbose {
			logger.Warn("无法解析工具提示，假定为最终答案",
				zap.Error(err),
				zap.String("response", resp),
			)
			color.Cyan("Unable to parse tool from prompt, assuming got final answer.\n\n", resp)
			color.Cyan("json marshal error: %s\n\n", err)
		}
		return cleanedResp, chatHistory, nil
	}

	iterations := 0
	if maxIterations <= 0 {
		maxIterations = defaultMaxIterations
	}
	for {
		iterations++
		logger.Debug("开始新的迭代",
			zap.Int("iteration", iterations),
			zap.String("thought", toolPrompt.Thought),
		)

		if verbose {
			color.Cyan("Thought: %s\n\n", toolPrompt.Thought)
		}

		if iterations > maxIterations {
			logger.Warn("达到最大迭代次数",
				zap.Int("maxIterations", maxIterations),
			)
			color.Red("Max iterations reached")
			return toolPrompt.FinalAnswer, chatHistory, nil
		}

		if toolPrompt.FinalAnswer != "" {
			logger.Info("获得最终答案",
				zap.String("finalAnswer", toolPrompt.FinalAnswer),
			)
			if verbose {
				color.Cyan("Final answer: %v\n\n", toolPrompt.FinalAnswer)
			}
			return toolPrompt.FinalAnswer, chatHistory, nil
		}

		if toolPrompt.Action.Name != "" {
			var observation string
			logger.Debug("执行工具",
				zap.String("tool", toolPrompt.Action.Name),
				zap.String("input", toolPrompt.Action.Input),
			)

			if verbose {
				color.Blue("Iteration %d): executing tool %s\n", iterations, toolPrompt.Action.Name)
				color.Cyan("Invoking %s tool with inputs: \n============\n%s\n============\n\n", toolPrompt.Action.Name, toolPrompt.Action.Input)
			}

			if toolFunc, ok := tools.CopilotTools[toolPrompt.Action.Name]; ok {
				ret, err := toolFunc(toolPrompt.Action.Input)
				observation = strings.TrimSpace(ret)
				if err != nil {
					logger.Error("工具执行失败",
						zap.String("tool", toolPrompt.Action.Name),
						zap.Error(err),
					)
					observation = fmt.Sprintf("Tool %s failed with error %s. Considering refine the inputs for the tool.", toolPrompt.Action.Name, ret)
				} else {
					logger.Debug("工具执行成功",
						zap.String("tool", toolPrompt.Action.Name),
						zap.String("observation", observation),
					)
				}
			} else {
				logger.Warn("工具不可用",
					zap.String("tool", toolPrompt.Action.Name),
				)
				observation = fmt.Sprintf("Tool %s is not available. Considering switch to other supported tools.", toolPrompt.Action.Name)
			}

			if verbose {
				color.Cyan("Observation: %s\n\n", observation)
			}

			// Constrict the prompt to the max tokens allowed by the model.
			// This is required because the tool may have generated a long output.
			observation = llms.ConstrictPrompt(observation, model, 1024)
			toolPrompt.Observation = observation
			assistantMessage, _ := json.Marshal(toolPrompt)
			chatHistory = append(chatHistory, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleUser,
				Content: string(assistantMessage),
			})
			// Constrict the chat history to the max tokens allowed by the model.
			// This is required because the chat history may have grown too large.
			chatHistory = llms.ConstrictMessages(chatHistory, model, maxTokens)

			// Start next iteration of LLM chat.
			if verbose {
				color.Blue("Iteration %d): chatting with LLM\n", iterations)
			}

			resp, err := client.Chat(model, maxTokens, chatHistory)
			if err != nil {
				logger.Error("对话完成失败",
					zap.Error(err),
				)
				return "", chatHistory, fmt.Errorf("chat completion error: %v", err)
			}

			chatHistory = append(chatHistory, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleAssistant,
				Content: string(resp),
			})
			if verbose {
				logger.Debug("LLM 中间响应",
					zap.String("response", resp),
				)
				color.Cyan("Intermediate response from LLM: %s\n\n", resp)
			}

			if err = json.Unmarshal([]byte(resp), &toolPrompt); err != nil {
				if verbose {
					logger.Warn("无法从 LLM 解析工具，总结最终答案",
						zap.Error(err),
					)
					color.Cyan("Unable to parse tools from LLM (%s), summarizing the final answer.\n\n", err.Error())
				}

				chatHistory = append(chatHistory, openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleUser,
					Content: "Summarize all the chat history and respond to original question with final answer",
				})

				resp, err = client.Chat(model, maxTokens, chatHistory)
				if err != nil {
					logger.Error("总结对话失败",
						zap.Error(err),
					)
					return "", chatHistory, fmt.Errorf("chat completion error: %v", err)
				}

				logger.Info("完成总结",
					zap.String("summary", resp),
				)
				
				// 尝试从响应中提取final_answer
				var finalResponse map[string]interface{}
				if err := json.Unmarshal([]byte(resp), &finalResponse); err == nil {
					if finalAnswer, ok := finalResponse["final_answer"].(string); ok && finalAnswer != "" {
						logger.Info("成功提取final_answer",
							zap.String("final_answer", finalAnswer),
						)
						return finalAnswer, chatHistory, nil
					}
				}
				
				return resp, chatHistory, nil
			}
		}
	}
}

func cleanJSON(input string) string {
	// 清理 Markdown 标记
	input = strings.TrimSpace(input)
	input = strings.TrimPrefix(input, "```json")
	input = strings.TrimPrefix(input, "```")
	input = strings.TrimSuffix(input, "```")
	
	// 尝试解析和重新格式化 JSON
	var temp interface{}
	if err := json.Unmarshal([]byte(input), &temp); err == nil {
		if cleanBytes, err := json.Marshal(temp); err == nil {
			return string(cleanBytes)
		}
	}
	
	// 如果 JSON 解析失败，进行基本的清理
	input = strings.ReplaceAll(input, "\n", " ")
	input = strings.ReplaceAll(input, "\r", " ")
	input = regexp.MustCompile(`\s+`).ReplaceAllString(input, " ")
	return strings.TrimSpace(input)
}
