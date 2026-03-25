package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	port := flag.Int("port", 8000, "Port for SSE transport")
	transport := flag.String("transport", "stdio", "Transport to use (stdio, sse)")
	flag.Parse()

	log.Println("Initializing server dependencies and environments.")
	available := GetAvailableModels()
	if len(available) == 0 {
		log.Println("WARNING: No AI models found! Please ensure models.json is valid and API key environment variables are set.")
	}

	s := server.NewMCPServer(
		"many-opinions",
		"0.1.0",
		server.WithToolCapabilities(true),
	)

	// ask_opinion
	s.AddTool(mcp.Tool{
		Name:        "ask_opinion",
		Description: "Get an opinion from an AI model.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"prompt": map[string]interface{}{
					"type":        "string",
					"description": "Your question or topic to get an opinion on.",
				},
				"model": map[string]interface{}{
					"type":        "string",
					"description": "LiteLLM model string (e.g. \"openai/gpt-4o\", \"gemini/gemini-2.5-flash\"). If omitted, uses best available.",
				},
				"personality": map[string]interface{}{
					"type":        "string",
					"description": "Response style — one of: honest, friend, coach, wise, creative.",
					"default":     "honest",
				},
				"temperature": map[string]interface{}{
					"type":        "number",
					"description": "Creativity level (0.0 = deterministic, 2.0 = very creative). Default 0.7.",
					"default":     0.7,
				},
				"max_tokens": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum response length. Default 4096.",
					"default":     4096,
				},
			},
			Required: []string{"prompt"},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := request.GetArguments()
		prompt, ok := args["prompt"].(string)
		if !ok {
			return mcp.NewToolResultError("prompt is required"), nil
		}
		model, _ := args["model"].(string)
		personality, ok := args["personality"].(string)
		if !ok || personality == "" {
			personality = "honest"
		}
		temperature := float32(0.7)
		if t, ok := args["temperature"].(float64); ok {
			temperature = float32(t)
		}
		maxTokens := 4096
		if m, ok := args["max_tokens"].(float64); ok {
			maxTokens = int(m)
		}

		opinion, err := getOpinion(ctx, prompt, model, personality, temperature, maxTokens)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(opinion), nil
	})

	// compare_opinions
	s.AddTool(mcp.Tool{
		Name:        "compare_opinions",
		Description: "Compare opinions from multiple AI models side by side.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"prompt": map[string]interface{}{
					"type":        "string",
					"description": "Your question or topic to compare opinions on.",
				},
				"models": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "string",
					},
					"description": "List of model strings to compare.",
				},
				"personality": map[string]interface{}{
					"type":        "string",
					"description": "Response style — one of: honest, friend, coach, wise, creative.",
					"default":     "honest",
				},
				"temperature": map[string]interface{}{
					"type":        "number",
					"description": "Creativity level (0.0-2.0). Default 0.7.",
					"default":     0.7,
				},
				"max_tokens": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum response length per model. Default 4096.",
					"default":     4096,
				},
			},
			Required: []string{"prompt"},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := request.GetArguments()
		prompt, ok := args["prompt"].(string)
		if !ok {
			return mcp.NewToolResultError("prompt is required"), nil
		}

		var models []string
		if mIfs, ok := args["models"].([]interface{}); ok {
			for _, mIf := range mIfs {
				if m, ok := mIf.(string); ok {
					models = append(models, m)
				}
			}
		}

		personality, ok := args["personality"].(string)
		if !ok || personality == "" {
			personality = "honest"
		}
		temperature := float32(0.7)
		if t, ok := args["temperature"].(float64); ok {
			temperature = float32(t)
		}
		maxTokens := 4096
		if m, ok := args["max_tokens"].(float64); ok {
			maxTokens = int(m)
		}

		opinion, err := compareOpinions(ctx, prompt, models, personality, temperature, maxTokens)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(opinion), nil
	})

	// collect_opinions
	s.AddTool(mcp.Tool{
		Name:        "collect_opinions",
		Description: "Poll multiple AI models for opinions, one from each provider.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"prompt": map[string]interface{}{
					"type":        "string",
					"description": "Your question or topic to collect opinions on.",
				},
				"count": map[string]interface{}{
					"type":        "integer",
					"description": "Number of AI models to poll. Default 3.",
					"default":     3,
				},
				"high_quality": map[string]interface{}{
					"type":        "boolean",
					"description": "If true, select only premium models (quality > 92).",
					"default":     false,
				},
				"personality": map[string]interface{}{
					"type":        "string",
					"description": "Response style — one of: honest, friend, coach, wise, creative.",
					"default":     "honest",
				},
				"temperature": map[string]interface{}{
					"type":        "number",
					"description": "Creativity level (0.0-2.0). Default 0.7.",
					"default":     0.7,
				},
				"max_tokens": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum response length per model. Default 4096.",
					"default":     4096,
				},
			},
			Required: []string{"prompt"},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := request.GetArguments()
		prompt, ok := args["prompt"].(string)
		if !ok {
			return mcp.NewToolResultError("prompt is required"), nil
		}
		count := 3
		if c, ok := args["count"].(float64); ok {
			count = int(c)
		}
		highQuality := false
		if hq, ok := args["high_quality"].(bool); ok {
			highQuality = hq
		}
		personality, ok := args["personality"].(string)
		if !ok || personality == "" {
			personality = "honest"
		}
		temperature := float32(0.7)
		if t, ok := args["temperature"].(float64); ok {
			temperature = float32(t)
		}
		maxTokens := 4096
		if m, ok := args["max_tokens"].(float64); ok {
			maxTokens = int(m)
		}

		opinion, err := collectOpinions(ctx, prompt, count, highQuality, personality, temperature, maxTokens)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(opinion), nil
	})

	// list_available_models
	s.AddTool(mcp.Tool{
		Name:        "list_available_models",
		Description: "List all configured AI models grouped by provider, with quality scores.",
		InputSchema: mcp.ToolInputSchema{
			Type:       "object",
			Properties: map[string]interface{}{},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText(listModels()), nil
	})

	// list_personalities
	s.AddTool(mcp.Tool{
		Name:        "list_personalities",
		Description: "List available personality styles for opinions.",
		InputSchema: mcp.ToolInputSchema{
			Type:       "object",
			Properties: map[string]interface{}{},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText(listPersonalities()), nil
	})

	// reload_model_config
	s.AddTool(mcp.Tool{
		Name:        "reload_model_config",
		Description: "Reload models from models.json. Call after editing the file to pick up changes.",
		InputSchema: mcp.ToolInputSchema{
			Type:       "object",
			Properties: map[string]interface{}{},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ReloadModels()
		catalogMutex.RLock()
		defer catalogMutex.RUnlock()
		return mcp.NewToolResultText(fmt.Sprintf("Reloaded %d models from models.json.", len(modelCatalog))), nil
	})

	log.Printf("Starting FastMCP server with transport='%s'", *transport)

	if *transport == "sse" {
		httpServer := server.NewStreamableHTTPServer(s)
		log.Printf("Streamable HTTP server listening on http://localhost:%d/mcp", *port)
		if err := httpServer.Start(fmt.Sprintf(":%d", *port)); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	} else {
		if err := server.ServeStdio(s); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}
}
