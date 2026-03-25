package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/generative-ai-go/genai"
	"github.com/liushuangls/go-anthropic/v2"
	"github.com/sashabaranov/go-openai"
	"google.golang.org/api/option"
)

var (
	ErrNoProviders  = fmt.Errorf("Error: No AI providers configured. Set API keys in your environment.")
	ErrNoModelFound = fmt.Errorf("Error: No models available at requested quality tier. Check your API keys and models.json.")
)

func getOpenAIClientForProvider(provider string) *openai.Client {
	var baseURL string
	switch provider {
	case "deepseek":
		baseURL = "https://api.deepseek.com/v1"
	case "openrouter":
		baseURL = "https://openrouter.ai/api/v1"
	case "xai":
		baseURL = "https://api.x.ai/v1"
	case "groq":
		baseURL = "https://api.groq.com/openai/v1"
	case "mistral":
		baseURL = "https://api.mistral.ai/v1"
	case "fireworks_ai":
		baseURL = "https://api.fireworks.ai/inference/v1"
	case "together_ai":
		baseURL = "https://api.together.xyz/v1"
	case "perplexity":
		baseURL = "https://api.perplexity.ai"
	default:
		baseURL = "" // Use default
	}

	apiKey := os.Getenv(ProviderEnvKeys[provider])
	config := openai.DefaultConfig(apiKey)
	if baseURL != "" {
		config.BaseURL = baseURL
	}
	return openai.NewClientWithConfig(config)
}

func getOpinion(ctx context.Context, prompt string, modelID string, personality string, temperature float32, maxTokens int) (string, error) {
	if modelID == "" {
		best := GetBestModel()
		if best == nil {
			return "", ErrNoProviders
		}
		modelID = best.ID
	}

	modelID = strings.TrimSpace(modelID)
	provider := GetProvider(modelID)
	if !IsProviderConfigured(provider) {
		return "", fmt.Errorf("Error: Provider '%s' is not configured. Set %s.", provider, ProviderEnvKeys[provider])
	}

	systemPrompt := Personalities["honest"]
	if val, ok := Personalities[personality]; ok {
		systemPrompt = val
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(LLMTimeout)*time.Second)
	defer cancel()

	log.Printf("[%s] Sending request (timeout=%ds, max_tokens=%d, temp=%.1f)", modelID, LLMTimeout, maxTokens, temperature)
	start := time.Now()

	modelName := modelID
	if strings.HasPrefix(modelID, provider+"/") {
		modelName = strings.TrimPrefix(modelID, provider+"/")
	}

	var content string
	var err error

	switch provider {
	case "anthropic":
		client := anthropic.NewClient(os.Getenv(ProviderEnvKeys[provider]))
		resp, anthErr := client.CreateMessages(timeoutCtx, anthropic.MessagesRequest{
			Model:       anthropic.Model(modelName),
			System:      systemPrompt,
			MaxTokens:   maxTokens,
			Temperature: &temperature,
			Messages: []anthropic.Message{
				anthropic.NewUserTextMessage(prompt),
			},
		})
		if anthErr != nil {
			err = anthErr
		} else {
			if len(resp.Content) > 0 {
				content = *resp.Content[0].Text
			}
		}

	case "gemini":
		client, genErr := genai.NewClient(ctx, option.WithAPIKey(os.Getenv(ProviderEnvKeys[provider])))
		if genErr != nil {
			err = genErr
			break
		}
		defer client.Close()
		geminiModel := client.GenerativeModel(modelName)
		if temperature > 0 {
			geminiModel.Temperature = &temperature
		}
		if maxTokens > 0 {
			toks := int32(maxTokens)
			geminiModel.MaxOutputTokens = &toks
		}
		if systemPrompt != "" {
			geminiModel.SystemInstruction = &genai.Content{
				Parts: []genai.Part{genai.Text(systemPrompt)},
			}
		}

		resp, genErr := geminiModel.GenerateContent(timeoutCtx, genai.Text(prompt))
		if genErr != nil {
			err = genErr
		} else {
			if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
				if txt, ok := resp.Candidates[0].Content.Parts[0].(genai.Text); ok {
					content = string(txt)
				}
			}
		}

	default: // Map to openai compatible
		client := getOpenAIClientForProvider(provider)
		resp, oaErr := client.CreateChatCompletion(timeoutCtx, openai.ChatCompletionRequest{
			Model: modelName,
			Messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
				{Role: openai.ChatMessageRoleUser, Content: prompt},
			},
			Temperature:         temperature,
			MaxCompletionTokens: maxTokens,
		})
		if oaErr != nil {
			err = oaErr
		} else {
			if len(resp.Choices) > 0 {
				content = resp.Choices[0].Message.Content
			}
		}
	}

	elapsed := time.Since(start)

	if err != nil {
		log.Printf("[%s] Error after %.1fs: %v", modelID, elapsed.Seconds(), err)
		return "", fmt.Errorf("Unexpected error calling %s: %w", modelID, err)
	}

	log.Printf("[%s] Response received in %.1fs (len=%d)", modelID, elapsed.Seconds(), len(content))
	return content, nil
}

func compareOpinions(ctx context.Context, prompt string, models []string, personality string, temperature float32, maxTokens int) (string, error) {
	if len(models) == 0 {
		available := GetSortedModels()
		seenProviders := make(map[string]bool)
		for _, m := range available {
			provider := GetProvider(m.ID)
			if !seenProviders[provider] {
				models = append(models, m.ID)
				seenProviders[provider] = true
			}
			if len(models) >= 3 {
				break
			}
		}
	}

	if len(models) == 0 {
		return "", ErrNoProviders
	}

	log.Printf("Comparing %d models: %s", len(models), strings.Join(models, ", "))
	start := time.Now()

	type resultStruct struct {
		model string
		text  string
		err   error
	}

	resultsChan := make(chan resultStruct, len(models))
	var wg sync.WaitGroup

	for _, m := range models {
		wg.Add(1)
		go func(modelID string) {
			defer wg.Done()
			text, err := getOpinion(ctx, prompt, modelID, personality, temperature, maxTokens)
			resultsChan <- resultStruct{model: modelID, text: text, err: err}
		}(m)
	}

	wg.Wait()
	close(resultsChan)

	elapsed := time.Since(start)
	log.Printf("All %d comparisons completed in %.1fs", len(models), elapsed.Seconds())

	// Reorder results to match input order
	resultMap := make(map[string]resultStruct)
	for res := range resultsChan {
		resultMap[res.model] = res
	}

	var parts []string
	for _, m := range models {
		res := resultMap[m]
		if res.err != nil {
			parts = append(parts, fmt.Sprintf("## %s\n\nError: %v\n", m, res.err))
		} else {
			parts = append(parts, fmt.Sprintf("## %s\n\n%s\n", m, res.text))
		}
	}

	return strings.Join(parts, "\n---\n\n"), nil
}

func selectModels(count int, highQuality bool) []string {
	threshold := 85
	if highQuality {
		threshold = 92
	}

	available := GetSortedModels()
	var candidates []Model
	for _, m := range available {
		if m.Quality > threshold {
			candidates = append(candidates, m)
		}
	}

	seenProviders := make(map[string]bool)
	var selected []string
	for _, m := range candidates {
		provider := GetProvider(m.ID)
		if !seenProviders[provider] {
			selected = append(selected, m.ID)
			seenProviders[provider] = true
		}
		if len(selected) >= count {
			break
		}
	}
	return selected
}

func collectOpinions(ctx context.Context, prompt string, count int, highQuality bool, personality string, temperature float32, maxTokens int) (string, error) {
	models := selectModels(count, highQuality)
	tierLabel := "standard"
	if highQuality {
		tierLabel = "high"
	}

	if len(models) == 0 {
		return "", ErrNoModelFound
	}

	log.Printf("Collecting %d opinions (%s quality): %s", len(models), tierLabel, strings.Join(models, ", "))
	start := time.Now()

	type resultStruct struct {
		model string
		text  string
		err   error
	}

	resultsChan := make(chan resultStruct, len(models))
	var wg sync.WaitGroup

	for _, m := range models {
		wg.Add(1)
		go func(modelID string) {
			defer wg.Done()
			text, err := getOpinion(ctx, prompt, modelID, personality, temperature, maxTokens)
			resultsChan <- resultStruct{model: modelID, text: text, err: err}
		}(m)
	}

	wg.Wait()
	close(resultsChan)

	elapsed := time.Since(start)
	successCount := 0

	resultMap := make(map[string]resultStruct)
	for res := range resultsChan {
		resultMap[res.model] = res
		if res.err == nil {
			successCount++
		}
	}

	log.Printf("Collected %d/%d opinions in %.1fs", successCount, len(models), elapsed.Seconds())

	parts := []string{fmt.Sprintf("*Collected %d opinions (%s quality tier)*\n", len(models), tierLabel)}
	for _, m := range models {
		res := resultMap[m]
		if res.err != nil {
			parts = append(parts, fmt.Sprintf("## %s\n\nError: %v\n", m, res.err))
		} else {
			parts = append(parts, fmt.Sprintf("## %s\n\n%s\n", m, res.text))
		}
	}

	return strings.Join(parts, "\n---\n\n"), nil
}

func listModels() string {
	available := GetSortedModels()
	if len(available) == 0 {
		return "No AI providers configured. Set API keys in your environment."
	}

	byProvider := make(map[string][]Model)
	var sortedProviders []string

	for _, m := range available {
		provider := GetProvider(m.ID)
		if _, exists := byProvider[provider]; !exists {
			sortedProviders = append(sortedProviders, provider)
		}
		byProvider[provider] = append(byProvider[provider], m)
	}
	sort.Strings(sortedProviders)

	var lines []string
	for _, p := range sortedProviders {
		lines = append(lines, fmt.Sprintf("### %s", p))
		for _, m := range byProvider[p] {
			lines = append(lines, fmt.Sprintf("  - `%s` — %s (quality: %d)", m.ID, m.Name, m.Quality))
		}
	}

	return strings.Join(lines, "\n")
}

func listPersonalities() string {
	var names []string
	for k := range Personalities {
		names = append(names, k)
	}
	sort.Strings(names)

	var lines []string
	for _, k := range names {
		desc := Personalities[k]
		if len(desc) > 80 {
			desc = desc[:80] + "..."
		}
		lines = append(lines, fmt.Sprintf("**%s**: %s", k, desc))
	}
	return strings.Join(lines, "\n")
}
