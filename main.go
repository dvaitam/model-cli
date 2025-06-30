package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Operation struct {
	Shell string `json:"shell,omitempty"`
	Edit  *Edit  `json:"edit,omitempty"`
	Done  bool   `json:"done,omitempty"`
}

type Edit struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func main() {
	provider := flag.String("provider", "openai", "Model provider: openai, gemini, xai, anthropic")
	model := flag.String("model", "gpt-3.5-turbo", "Model name")
	prompt := flag.String("prompt", "", "Task prompt")
	flag.Parse()

	if *prompt == "" {
		fmt.Println("prompt required")
		os.Exit(1)
	}

	systemPrompt := `You are a coding agent that generates JSON instructions. For each step respond with JSON array of operations. Available operations:\n{"shell": "<command>"} to run shell commands, {"edit": {"path": "<file>", "content": "<text>"}} to write files, or {"done": true} when finished.`

	convo := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: *prompt},
	}

	for i := 0; i < 20; i++ {
		resp, err := callProvider(*provider, *model, convo)
		if err != nil {
			fmt.Println("error calling provider:", err)
			return
		}

		ops, err := parseOps(resp)
		if err != nil {
			fmt.Println("parse error:", err)
			return
		}

		if len(ops) == 1 && ops[0].Done {
			fmt.Println("Task complete")
			return
		}

		result := executeOps(ops)

		convo = append(convo, Message{Role: "assistant", Content: resp})
		convo = append(convo, Message{Role: "user", Content: result})
	}
}

func parseOps(resp string) ([]Operation, error) {
	var ops []Operation
	if err := json.Unmarshal([]byte(resp), &ops); err != nil {
		return nil, err
	}
	return ops, nil
}

func executeOps(ops []Operation) string {
	var out string
	for _, op := range ops {
		if op.Shell != "" {
			cmd := exec.Command("bash", "-c", op.Shell)
			b, err := cmd.CombinedOutput()
			if err != nil {
				out += fmt.Sprintf("$ %s\n%s\n(error: %v)\n", op.Shell, string(b), err)
			} else {
				out += fmt.Sprintf("$ %s\n%s\n", op.Shell, string(b))
			}
		}
		if op.Edit != nil {
			ioutil.WriteFile(op.Edit.Path, []byte(op.Edit.Content), 0644)
			out += fmt.Sprintf("edited %s\n", op.Edit.Path)
		}
	}
	return out
}

func callProvider(provider, model string, convo []Message) (string, error) {
	switch provider {
	case "openai":
		return callOpenAI(model, convo)
	case "anthropic":
		return callAnthropic(model, convo)
	case "gemini":
		return callGemini(model, convo)
	case "xai":
		return callXAI(model, convo)
	default:
		return "", errors.New("unknown provider")
	}
}

func callOpenAI(model string, convo []Message) (string, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return "", errors.New("OPENAI_API_KEY not set")
	}
	reqBody := map[string]interface{}{
		"model":    model,
		"messages": convo,
	}
	b, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	rbody, _ := io.ReadAll(resp.Body)
	var data struct {
		Choices []struct {
			Message Message `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(rbody, &data); err != nil {
		return "", err
	}
	if len(data.Choices) == 0 {
		return "", errors.New("no choices")
	}
	return data.Choices[0].Message.Content, nil
}

func callAnthropic(model string, convo []Message) (string, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return "", errors.New("ANTHROPIC_API_KEY not set")
	}
	reqBody := map[string]interface{}{
		"model":    model,
		"messages": convo,
	}
	b, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(b))
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	rbody, _ := io.ReadAll(resp.Body)
	var data struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(rbody, &data); err != nil {
		return "", err
	}
	return data.Content, nil
}

func callGemini(model string, convo []Message) (string, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return "", errors.New("GEMINI_API_KEY not set")
	}
	var contents []map[string]interface{}
	for _, m := range convo {
		contents = append(contents, map[string]interface{}{
			"role":  m.Role,
			"parts": []map[string]string{{"text": m.Content}},
		})
	}
	reqBody := map[string]interface{}{"contents": contents}
	b, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, apiKey)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	rbody, _ := io.ReadAll(resp.Body)
	var data struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(rbody, &data); err != nil {
		return "", err
	}
	if len(data.Candidates) == 0 || len(data.Candidates[0].Content.Parts) == 0 {
		return "", errors.New("no response")
	}
	return data.Candidates[0].Content.Parts[0].Text, nil
}

func callXAI(model string, convo []Message) (string, error) {
	apiKey := os.Getenv("XAI_API_KEY")
	if apiKey == "" {
		return "", errors.New("XAI_API_KEY not set")
	}
	reqBody := map[string]interface{}{
		"model":    model,
		"messages": convo,
	}
	b, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "https://api.x.ai/v1/chat/completions", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	rbody, _ := io.ReadAll(resp.Body)
	var data struct {
		Choices []struct {
			Message Message `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(rbody, &data); err != nil {
		return "", err
	}
	if len(data.Choices) == 0 {
		return "", errors.New("no choices")
	}
	return data.Choices[0].Message.Content, nil
}
