package usage

import (
	"bytes"
	"encoding/json"
	"strings"
)

type openAIResponse struct {
	Model string       `json:"model"`
	Usage *openAIUsage `json:"usage"`
}

type openAIUsage struct {
	PromptTokens            int                `json:"prompt_tokens"`
	CompletionTokens        int                `json:"completion_tokens"`
	TotalTokens             int                `json:"total_tokens"`
	PromptTokensDetails     openAIPromptDetail `json:"prompt_tokens_details"`
	CompletionTokensDetails openAICompDetail   `json:"completion_tokens_details"`
}

type openAIPromptDetail struct {
	CachedTokens int `json:"cached_tokens"`
}

type openAICompDetail struct {
	ReasoningTokens int `json:"reasoning_tokens"`
}

func ParseNonStream(body []byte) (ParsedUsage, bool, error) {
	var response openAIResponse
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&response); err != nil {
		return ParsedUsage{}, false, err
	}
	if response.Usage == nil {
		return ParsedUsage{ModelActual: response.Model}, false, nil
	}
	return ParsedUsage{
		ModelActual: response.Model,
		Tokens:      usageTokens(response.Usage),
	}, true, nil
}

func ParseStream(tail []byte) (ParsedUsage, bool, error) {
	var parsed ParsedUsage
	found := false
	for _, line := range strings.Split(string(tail), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		var chunk openAIResponse
		decoder := json.NewDecoder(strings.NewReader(payload))
		decoder.UseNumber()
		if err := decoder.Decode(&chunk); err != nil {
			return parsed, found, err
		}
		if chunk.Model != "" {
			parsed.ModelActual = chunk.Model
		}
		if chunk.Usage != nil {
			parsed.Tokens = usageTokens(chunk.Usage)
			found = true
		}
	}
	return parsed, found, nil
}

func usageTokens(usage *openAIUsage) TokenBreakdown {
	tokens := TokenBreakdown{
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		ReasoningTokens:  usage.CompletionTokensDetails.ReasoningTokens,
		CachedTokens:     usage.PromptTokensDetails.CachedTokens,
		TotalTokens:      usage.TotalTokens,
	}
	if tokens.TotalTokens == 0 {
		tokens.TotalTokens = tokens.PromptTokens + tokens.CompletionTokens + tokens.ReasoningTokens
	}
	return tokens
}
