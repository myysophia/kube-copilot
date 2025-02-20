/*
Copyright 2023 - Present, Pengfei Ni

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
package tools

// Tool 是一个接受输入并返回输出的函数类型
type Tool func(input string) (string, error)

// function call ，可以理解这里是hook点，可以在这里添加自己的工具
var CopilotTools = map[string]Tool{
	"search":  GoogleSearch,
	"python":  PythonREPL,
	"trivy":   Trivy,
	"kubectl": Kubectl,
}

// ToolPrompt 定义了与 LLM 交互的 JSON 格式
type ToolPrompt struct {
	Question string   `json:"question"` // 用户输入的问题
	Thought  string   `json:"thought"`  // AI 的思考过程
	Action   struct { // 需要执行的动作
		Name  string `json:"name"`  // 工具名称
		Input string `json:"input"` // 工具输入
	} `json:"action"`
	Observation string `json:"observation"`  // 工具执行结果
	FinalAnswer string `json:"final_answer"` // 最终答案
}
