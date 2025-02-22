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
package tools

import (
	"fmt"
	"github.com/fatih/color"
	"os/exec"
	"strings"
)

// PythonREPL runs the given Python script and returns the output.
func PythonREPL(script string) (string, error) {
	//exec.Command("k8s-env")
	//cmd := exec.Command("cd ~/k8s/python-cli && source k8s-env/bin/activate && python3", "-c", script)
	//cmd := exec.Command("python3", "-c", script)
	//color.Cyan("Python scripts is:%s", cmd)
	escapedScript := strings.ReplaceAll(script, "\"", "\\\"")
	cmdStr := fmt.Sprintf("cd ~/k8s/python-cli && source k8s-env/bin/activate && python3 -c \"%s\"", escapedScript)
	cmd := exec.Command("bash", "-c", cmdStr)
	color.Cyan("Python scripts is: %s", cmdStr)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return strings.TrimSpace(string(output)), err
	}

	return strings.TrimSpace(string(output)), nil
}

// SwitchK8sEnv 切换到指定的 Kubernetes 环境
func SwitchK8sEnv(envName string) error {
	// 假设 k8s-env 是一个命令，用于切换环境
	cmd := exec.Command("k8s-env", "switch", envName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to switch to %s: %s, output: %s", envName, err, output)
	}
	fmt.Printf("Switched to %s: %s\n", envName, output)
	return nil
}
