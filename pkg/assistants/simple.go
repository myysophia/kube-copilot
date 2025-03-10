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
	"github.com/feiskyer/kube-copilot/pkg/utils"
	"github.com/sashabaranov/go-openai"
	"go.uber.org/zap"
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
	//cleanedResp := cleanJSON(resp)
	logger.Debug("清理后的响应",
		zap.String("response", resp),
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
	if err = json.Unmarshal([]byte(resp), &toolPrompt); err != nil {
		if verbose {
			logger.Warn("无法解析工具提示，假定为最终答案",
				zap.Error(err),
				zap.String("response", resp),
			)
			color.Cyan("Unable to parse tool from prompt, assuming got final answer.\n\n", resp)
			color.Cyan("json marshal error: %s\n\n", err)
		}
		return resp, chatHistory, nil
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

		// 检查final_answer是否为有效值（不是模板或占位符）
		if toolPrompt.FinalAnswer != "" && !isTemplateValue(toolPrompt.FinalAnswer) {
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

				// 尝试从响应中提取final_answer并处理格式
				// 这里处理LLM返回的JSON响应，确保只返回final_answer部分
				var finalResponse map[string]interface{}
				if err := json.Unmarshal([]byte(resp), &finalResponse); err == nil {
					if finalAnswer, ok := finalResponse["final_answer"].(string); ok && finalAnswer != "" {
						logger.Info("成功提取final_answer",
							zap.String("final_answer", finalAnswer),
						)
						// 创建只包含final_answer的新响应对象
						// 这样可以确保返回的JSON格式统一且简洁
						//cleanResp := map[string]interface{}{
						//	"final_answer": finalAnswer,
						//}
						// 将清理后的响应重新序列化为JSON字符串
						//cleanJSON, err := json.Marshal(cleanResp)
						if err == nil {
							return string(resp), chatHistory, nil
						}
						// 如果JSON序列化失败，直接返回原始的final_answer
						return finalAnswer, chatHistory, nil
					}
				}

				// 如果无法直接提取final_answer，尝试清理原始响应
				//cleanedResp := cleanJSON(resp)
				return resp, chatHistory, nil
			}
		}
	}
}

// AssistantWithConfig is the AI assistant with custom configuration.
func AssistantWithConfig(model string, prompts []openai.ChatCompletionMessage, maxTokens int, countTokens bool, verbose bool, maxIterations int, apiKey string, baseUrl string) (result string, chatHistory []openai.ChatCompletionMessage, err error) {
	// 获取性能统计工具
	perfStats := utils.GetPerfStats()
	// 开始整体执行计时
	defer perfStats.TraceFunc("assistant_total")()

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

	// 开始创建客户端计时
	perfStats.StartTimer("assistant_create_client")

	client, err := llms.NewOpenAIClient(apiKey, baseUrl)

	// 停止创建客户端计时
	clientDuration := perfStats.StopTimer("assistant_create_client")
	logger.Debug("创建OpenAI客户端完成",
		zap.Duration("duration", clientDuration),
	)

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

	// 开始第一轮对话计时
	perfStats.StartTimer("assistant_first_chat")

	resp, err := client.Chat(model, maxTokens, chatHistory)

	// 停止第一轮对话计时
	chatDuration := perfStats.StopTimer("assistant_first_chat")
	logger.Debug("第一轮对话完成",
		zap.Duration("duration", chatDuration),
	)

	// 开始JSON清理计时
	perfStats.StartTimer("assistant_clean_json")

	// 停止JSON清理计时
	cleanDuration := perfStats.StopTimer("assistant_clean_json")
	logger.Debug("JSON清理完成",
		zap.Duration("duration", cleanDuration),
		zap.String("response", resp),
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

	// 开始解析工具提示计时
	perfStats.StartTimer("assistant_parse_tool_prompt")

	var toolPrompt tools.ToolPrompt
	if err = json.Unmarshal([]byte(resp), &toolPrompt); err != nil {
		// 停止解析工具提示计时
		parseDuration := perfStats.StopTimer("assistant_parse_tool_prompt")
		logger.Debug("解析工具提示失败",
			zap.Duration("duration", parseDuration),
			zap.Error(err),
		)

		if verbose {
			logger.Warn("无法解析工具提示，假定为最终答案",
				zap.Error(err),
				zap.String("response", resp),
			)
			color.Cyan("Unable to parse tool from prompt, assuming got final answer.\n\n", resp)
			color.Cyan("json marshal error: %s\n\n", err)
		}
		return resp, chatHistory, nil
	}

	// 停止解析工具提示计时
	parseDuration := perfStats.StopTimer("assistant_parse_tool_prompt")
	logger.Debug("解析工具提示成功",
		zap.Duration("duration", parseDuration),
	)

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

		// 检查final_answer是否为有效值（不是模板或占位符）
		if toolPrompt.FinalAnswer != "" && !isTemplateValue(toolPrompt.FinalAnswer) {
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

			// 开始工具执行计时
			perfStats.StartTimer("assistant_tool_" + toolPrompt.Action.Name)

			if toolFunc, ok := tools.CopilotTools[toolPrompt.Action.Name]; ok {
				ret, err := toolFunc(toolPrompt.Action.Input)
				observation = strings.TrimSpace(ret)

				// 停止工具执行计时
				toolDuration := perfStats.StopTimer("assistant_tool_" + toolPrompt.Action.Name)

				if err != nil {
					logger.Error("工具执行失败",
						zap.String("tool", toolPrompt.Action.Name),
						zap.Error(err),
						zap.Duration("duration", toolDuration),
					)
					observation = fmt.Sprintf("Tool %s failed with error %s. Considering refine the inputs for the tool.", toolPrompt.Action.Name, ret)
				} else {
					logger.Debug("工具执行成功",
						zap.String("tool", toolPrompt.Action.Name),
						zap.String("observation", observation),
						zap.Duration("duration", toolDuration),
					)
				}
			} else {
				// 停止工具执行计时（工具不可用的情况）
				toolDuration := perfStats.StopTimer("assistant_tool_" + toolPrompt.Action.Name)

				logger.Warn("工具不可用",
					zap.String("tool", toolPrompt.Action.Name),
					zap.Duration("duration", toolDuration),
				)
				observation = fmt.Sprintf("Tool %s is not available. Considering switch to other supported tools.", toolPrompt.Action.Name)
			}

			if verbose {
				color.Cyan("Observation: %s\n\n", observation)
			}

			// 开始消息构建计时
			perfStats.StartTimer("assistant_construct_message")

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

			// 停止消息构建计时
			constructDuration := perfStats.StopTimer("assistant_construct_message")
			logger.Debug("消息构建完成",
				zap.Duration("duration", constructDuration),
			)

			// Start next iteration of LLM chat.
			if verbose {
				color.Blue("Iteration %d): chatting with LLM\n", iterations)
			}

			// 开始中间对话计时
			perfStats.StartTimer("assistant_intermediate_chat")

			resp, err := client.Chat(model, maxTokens, chatHistory)

			// 停止中间对话计时
			intermediateChatDuration := perfStats.StopTimer("assistant_intermediate_chat")
			logger.Debug("中间对话完成",
				zap.Duration("duration", intermediateChatDuration),
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
				logger.Debug("LLM 中间响应",
					zap.String("response", resp),
				)
				color.Cyan("Intermediate response from LLM: %s\n\n", resp)
			}

			// 开始解析中间响应计时
			perfStats.StartTimer("assistant_parse_intermediate")

			// extract the tool prompt from the LLM response.
			if err = json.Unmarshal([]byte(resp), &toolPrompt); err != nil {
				// 停止解析中间响应计时
				parseIntermediateDuration := perfStats.StopTimer("assistant_parse_intermediate")
				logger.Debug("解析中间响应失败",
					zap.Duration("duration", parseIntermediateDuration),
					zap.Error(err),
				)

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

				// 开始总结对话计时
				perfStats.StartTimer("assistant_summarize")

				resp, err = client.Chat(model, maxTokens, chatHistory)

				// 停止总结对话计时
				summarizeDuration := perfStats.StopTimer("assistant_summarize")
				logger.Debug("总结对话完成",
					zap.Duration("duration", summarizeDuration),
				)

				if err != nil {
					logger.Error("总结对话失败",
						zap.Error(err),
					)
					return "", chatHistory, fmt.Errorf("chat completion error: %v", err)
				}

				logger.Info("完成总结",
					zap.String("summary", resp),
				)

				// 尝试从响应中提取final_answer并处理格式
				// 这里处理LLM返回的JSON响应，确保只返回final_answer部分
				var finalResponse map[string]interface{}
				if err := json.Unmarshal([]byte(resp), &finalResponse); err == nil {
					if finalAnswer, ok := finalResponse["final_answer"].(string); ok && finalAnswer != "" {
						logger.Info("成功提取final_answer",
							zap.String("final_answer", finalAnswer),
						)
						if err == nil {
							return string(resp), chatHistory, nil
						}
						// 如果JSON序列化失败，直接返回原始的final_answer
						return finalAnswer, chatHistory, nil
					}
				}
				return resp, chatHistory, nil
			} else {
				// 停止解析中间响应计时
				parseIntermediateDuration := perfStats.StopTimer("assistant_parse_intermediate")
				logger.Debug("解析中间响应成功",
					zap.Duration("duration", parseIntermediateDuration),
				)
			}
		}
	}
}

// isTemplateValue 检查字符串是否为模板值或占位符
// 参数：
//   - value: 要检查的字符串
// 返回：
//   - bool: 如果是模板值或占位符则返回true，否则返回false
func isTemplateValue(value string) bool {
	// 检查常见的模板值模式
	templatePatterns := []string{
		"<最终答案",
		"<final_answer",
		"<Final answer",
		"<最终回答",
		"<回答",
		"<答案",
		"使用 Markdown 格式",
		"使用Markdown格式",
		"换行符用 \\n 表示",
		"换行符用\\n表示",
	}
	
	// 如果值很短，可能是占位符
	if len(value) < 10 {
		return true
	}
	
	// 检查是否包含模板模式
	for _, pattern := range templatePatterns {
		if strings.Contains(value, pattern) {
			return true
		}
	}
	
	// 检查是否只包含指导性文本而没有实际内容
	if strings.Contains(value, "<") && strings.Contains(value, ">") {
		return true
	}
	
	return false
}
