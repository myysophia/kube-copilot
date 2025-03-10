# Kubernetes Copilot

Kubernetes Copilot powered by LLM, which leverages advanced language models to streamline and enhance Kubernetes cluster management. This tool integrates seamlessly with your existing Kubernetes setup, providing intelligent automation, diagnostics, and manifest generation capabilities. By utilizing the power of AI, Kubernetes Copilot simplifies complex operations and helps maintain the health and security of your Kubernetes workloads.

## Features

- Automate Kubernetes cluster operations using ChatGPT (GPT-4 or GPT-3.5).
- Diagnose and analyze potential issues for Kubernetes workloads.
- Generate Kubernetes manifests based on provided prompt instructions.
- Utilize native `kubectl` and `trivy` commands for Kubernetes cluster access and security vulnerability scanning.
- Access the web and perform Google searches without leaving the terminal.

## Installation

Install the kube-copilot CLI with the following command:

```sh
go install github.com/feiskyer/kube-copilot/cmd/kube-copilot@latest
```

## Quick Start

Setup the following environment variables:

- Ensure [`kubectl`](https://kubernetes.io/docs/tasks/tools/install-kubectl-linux/) is installed on the local machine and the kubeconfig file is configured for Kubernetes cluster access.
- Install [`trivy`](https://github.com/aquasecurity/trivy) to assess container image security issues (only required for the `audit` command).
- Set the OpenAI [API key](https://platform.openai.com/account/api-keys) as the `OPENAI_API_KEY` environment variable to enable ChatGPT functionality.

Then run the following commands directly in the terminal:

```sh
Kubernetes Copilot powered by OpenAI

Usage:
  kube-copilot [command]

Available Commands:
  analyze     Analyze issues for a given resource
  audit       Audit security issues for a Pod
  completion  Generate the autocompletion script for the specified shell
  diagnose    Diagnose problems for a Pod
  execute     Execute operations based on prompt instructions
  generate    Generate Kubernetes manifests
  help        Help about any command

Flags:
  -c, --count-tokens     Print tokens count
  -h, --help             help for kube-copilot
  -t, --max-tokens int   Max tokens for the GPT model (default 1024)
  -m, --model string     OpenAI model to use (default "gpt-4")
  -v, --verbose          Enable verbose output (default true)

Use "kube-copilot [command] --help" for more information about a command.
```

## LLM Integrations

<details>
<summary>OpenAI</summary>

Set the OpenAI [API key](https://platform.openai.com/account/api-keys) as the `OPENAI_API_KEY` environment variable to enable OpenAI functionality.
</details>

<details>
<summary>Azure OpenAI</summary>

For [Azure OpenAI service](https://learn.microsoft.com/en-us/azure/cognitive-services/openai/quickstart?tabs=command-line&pivots=rest-api#retrieve-key-and-endpoint), set the following environment variables:

- `OPENAI_API_KEY=<your-api-key>`
- `OPENAI_API_TYPE=azure`
- `OPENAI_API_BASE=https://<replace-this>.openai.azure.com/`
</details>

<details>
<summary>Ollama or other OpenAI compatible LLMs</summary>

For Ollama or other OpenAI compatible LLMs, set the following environment variables:

- `OPENAI_API_KEY=<your-api-key>`
- `OPENAI_API_BASE='http://localhost:11434/v1'` (or your own base URL)
</details>

## Key Features

<details>
<summary>Audit Security Issues for Pod</summary>

`kube-copilot audit POD [NAMESPACE]` will audit security issues for a Pod:

```sh
Usage: kube-copilot audit [OPTIONS] POD [NAMESPACE]

  audit security issues for a Pod

Options:
  --verbose      Enable verbose information of copilot execution steps
  --model MODEL  OpenAI model to use for copilot execution, default is gpt-4
  --help         Show this message and exit.
```
</details>


<details>
<summary>Diagnose Problems for Pod</summary>

`kube-copilot diagnose POD [NAMESPACE]` will diagnose problems for a Pod:

```sh
Usage: kube-copilot diagnose [OPTIONS] POD [NAMESPACE]

  diagnose problems for a Pod

Options:
  --verbose      Enable verbose information of copilot execution steps
  --model MODEL  OpenAI model to use for copilot execution, default is gpt-4
  --help         Show this message and exit.
```
</details>

<details>
<summary>Analyze Potential Issues for k8s Object</summary>

`kube-copilot analyze RESOURCE NAME [NAMESPACE]` will analyze potential issues for the given resource object:

```sh
Usage: kube-copilot analyze [OPTIONS] RESOURCE NAME [NAMESPACE]

  analyze issues for a given resource

Options:
  --verbose     Enable verbose information of copilot execution steps
  --model TEXT  OpenAI model to use for copilot execution, default is gpt-4
  --help        Show this message and exit.
```
</details>

<details>
<summary>Execute Operations Based on Prompt Instructions</summary>

`kube-copilot execute INSTRUCTIONS` will execute operations based on prompt instructions.
It could also be used to ask any questions.

```sh
Usage: kube-copilot execute [OPTIONS] INSTRUCTIONS

  execute operations based on prompt instructions

Options:
  --verbose      Enable verbose information of copilot execution steps
  --model MODEL  OpenAI model to use for copilot execution, default is gpt-4
  --help         Show this message and exit.
```
</details>

<details>
<summary>Generate Kubernetes Manifests</summary>

Use the `kube-copilot generate` command to create Kubernetes manifests based on
the provided prompt instructions. After generating the manifests, you will be
prompted to confirm whether you want to apply them.

```sh
Usage: kube-copilot generate [OPTIONS] INSTRUCTIONS

  generate Kubernetes manifests

Options:
  --verbose     Enable verbose information of copilot execution steps
  --model TEXT  OpenAI model to use for copilot execution, default is gpt-4
  --help        Show this message and exit.
```
</details>

## Integrations

<details>
<summary>Google Search</summary>

Large language models are trained with outdated data, and hence may lack the most current information or miss out on recent developments. This is where Google Search becomes an optional tool. By integrating real-time search capabilities, LLMs can access the latest data, ensuring that responses are not only accurate but also up-to-date.

To enable it, set `GOOGLE_API_KEY` and `GOOGLE_CSE_ID` (obtain API key from [Google Cloud](https://cloud.google.com/docs/authentication/api-keys?visit_id=638154888929258210-4085587461) and CSE ID from [Google CSE](http://www.google.com/cse/)).
</details>

## Python Version

Please refer [feiskyer/kube-copilot-python](https://github.com/feiskyer/kube-copilot-python) for the Python implementation of the same project.

## Contribution

The project is opensource at github [feiskyer/kube-copilot](https://github.com/feiskyer/kube-copilot) (Go) and [feiskyer/kube-copilot-python](https://github.com/feiskyer/kube-copilot-python) (Python) with Apache License.

If you would like to contribute to the project, please follow these guidelines:

1. Fork the repository and clone it to your local machine.
2. Create a new branch for your changes.
3. Make your changes and commit them with a descriptive commit message.
4. Push your changes to your forked repository.
5. Open a pull request to the main repository.

## Deployment

### Local Run

1. Clone the repository:
```bash
git clone https://github.com/YOUR_USERNAME/kube-copilot.git
cd kube-copilot
```

2. Build:
```bash
go build ./cmd/kube-copilot
```

3. Run:
```bash
./kube-copilot server --port 8080 --jwt-key your_jwt_key
```

### Docker Run

```bash
docker pull ghcr.io/YOUR_USERNAME/kube-copilot:latest
docker run -p 8080:8080 ghcr.io/YOUR_USERNAME/kube-copilot:latest server --jwt-key your_jwt_key
```

### Kubernetes Deployment

1. Create namespace:
```bash
kubectl create namespace kube-copilot
```

2. Create JWT key:
```bash
# Generate random key
JWT_KEY=$(openssl rand -base64 32)
# Create secret
kubectl create secret generic kube-copilot-secret \
  --from-literal=jwt-key=$JWT_KEY \
  -n kube-copilot
```

3. Deploy service:
```bash
# Modify deploy/kubernetes/deployment.yaml with image address
kubectl apply -f deploy/kubernetes/deployment.yaml
```

4. Verify deployment:
```bash
kubectl get pods -n kube-copilot
```

## Configuration

### Environment Variables

- `GIN_MODE`: Gin framework running mode, set to "release" for production
- `JWT_KEY`: JWT signing key

### Command Line Arguments

- `--port`: Service listening port (default: 8080)
- `--jwt-key`: JWT signing key (required)

## CI/CD

Project uses GitHub Actions for continuous integration and deployment:

- Build image automatically when code is pushed to main branch
- Build and push version image automatically when new tag (release) is created
- Supports AMD64 and ARM64 architectures

## Contribution Guidelines

1. Fork this repository
2. Create feature branch: `git checkout -b feature/my-feature`
3. Commit changes: `git commit -am 'Add new feature'`
4. Push branch: `git push origin feature/my-feature`
5. Submit Pull Request

## License

Apache License 2.0
