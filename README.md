# model-cli

`model-cli` is a command line agent that interacts with different AI model providers (OpenAI, Gemini, xAI, Anthropic) to perform coding tasks. The tool sends your prompt to the chosen model which responds with JSON instructions such as shell commands or file edits. `model-cli` executes these instructions and loops until the model signals completion.

## Usage

```bash
model-cli --provider openai --model gpt-3.5-turbo --prompt "Create hello world"
```

Supported providers require environment variables for API keys:

- `OPENAI_API_KEY`
- `GEMINI_API_KEY`
- `XAI_API_KEY`
- `ANTHROPIC_API_KEY`

Each response from the model must be a JSON array of operations. Examples:

```json
[{"shell":"echo 'hello'"}]
[{"edit":{"path":"main.go","content":"package main"}}]
[{"done":true}]
```

The system prompt enforces this structure automatically.
