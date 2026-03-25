package main

import (
	_ "embed"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/joho/godotenv"
)

//go:embed models.json
var defaultModels []byte

var (
	Personalities = map[string]string{
		"honest":   "You are a direct, factual advisor. Be brutally honest and concise. No sugarcoating. If something is bad, say so. Provide clear, actionable feedback.",
		"friend":   "You are an empathetic, supportive friend. Validate feelings while still being helpful. Acknowledge feelings before giving advice.",
		"coach":    "You are a demanding but encouraging coach. Push for action and accountability. Focus on growth and potential.",
		"wise":     "You are a wise elder. Speak calmly and thoughtfully. Be profound and measured. Offer perspective that transcends the immediate situation.",
		"creative": "You are an unconventional thinker. Look for surprising angles and innovative ideas. Think laterally.",
	}

	ProviderEnvKeys = map[string]string{
		"openai":       "OPENAI_API_KEY",
		"anthropic":    "ANTHROPIC_API_KEY",
		"gemini":       "GEMINI_API_KEY",
		"xai":          "XAI_API_KEY",
		"deepseek":     "DEEPSEEK_API_KEY",
		"mistral":      "MISTRAL_API_KEY",
		"groq":         "GROQ_API_KEY",
		"cohere":       "COHERE_API_KEY",
		"together_ai":  "TOGETHERAI_API_KEY",
		"perplexity":   "PERPLEXITYAI_API_KEY",
		"openrouter":   "OPENROUTER_API_KEY",
		"fireworks_ai": "FIREWORKS_AI_API_KEY",
	}

	modelCatalog []Model
	catalogMutex sync.RWMutex
	LLMTimeout   int
)

type Model struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Quality int    `json:"quality"`
}

type modelsFile struct {
	Models []Model `json:"models"`
}

func init() {
	// Load .env file if present (does not override existing env vars)
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using existing environment variables.")
	}

	timeoutStr := os.Getenv("LLM_TIMEOUT")
	if timeoutStr == "" {
		LLMTimeout = 120
	} else {
		val, err := strconv.Atoi(timeoutStr)
		if err != nil {
			log.Println("Invalid LLM_TIMEOUT environment variable. Defaulting to 120s")
			LLMTimeout = 120
		} else {
			LLMTimeout = val
		}
	}

	ReloadModels()
}

func loadModelCatalog() []Model {
	var data []byte
	var err error

	// 1. Try environment variable
	if envPath := os.Getenv("MANY_OPINIONS_MODELS_PATH"); envPath != "" {
		data, err = os.ReadFile(envPath)
		if err != nil {
			log.Printf("Could not load from MANY_OPINIONS_MODELS_PATH (%s): %v", envPath, err)
		}
	}

	// 2. Try adjacent to executable
	if data == nil {
		if executable, e := os.Executable(); e == nil {
			modelsPath := filepath.Join(filepath.Dir(executable), "models.json")
			if d, e := os.ReadFile(modelsPath); e == nil {
				data = d
			}
		}
	}

	// 3. Try current working directory
	if data == nil {
		if d, e := os.ReadFile("models.json"); e == nil {
			data = d
		}
	}

	// 4. Fallback to embedded models.json
	if data == nil {
		data = defaultModels
	}

	var parsed modelsFile
	if err := json.Unmarshal(data, &parsed); err != nil {
		log.Printf("Invalid format in models: %v", err)
		return []Model{}
	}

	for i := range parsed.Models {
		if parsed.Models[i].Quality == 0 {
			parsed.Models[i].Quality = 80
		}
	}

	log.Printf("Successfully loaded %d models.", len(parsed.Models))
	return parsed.Models
}

func ReloadModels() {
	newCatalog := loadModelCatalog()
	catalogMutex.Lock()
	defer catalogMutex.Unlock()
	modelCatalog = newCatalog
}

// GetProvider Extracts the provider prefix from a litellm-like model string.
func GetProvider(model string) string {
	parts := strings.SplitN(model, "/", 2)
	if len(parts) > 1 {
		return parts[0]
	}
	return model
}

func IsProviderConfigured(provider string) bool {
	envKey, exists := ProviderEnvKeys[provider]
	if !exists {
		return false
	}
	return os.Getenv(envKey) != ""
}

func GetAvailableModels() []Model {
	catalogMutex.RLock()
	defer catalogMutex.RUnlock()

	var available []Model
	for _, m := range modelCatalog {
		if IsProviderConfigured(GetProvider(m.ID)) {
			available = append(available, m)
		}
	}
	return available
}

func GetBestModel() *Model {
	available := GetAvailableModels()
	if len(available) == 0 {
		return nil
	}

	best := available[0]
	for _, m := range available {
		if m.Quality > best.Quality {
			best = m
		}
	}
	return &best
}

// ModelsByQuality is a utility type to sort models dynamically
type ModelsByQuality []Model

func (a ModelsByQuality) Len() int           { return len(a) }
func (a ModelsByQuality) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ModelsByQuality) Less(i, j int) bool { return a[i].Quality < a[j].Quality }

func GetSortedModels() []Model {
	available := GetAvailableModels()
	sort.Sort(sort.Reverse(ModelsByQuality(available)))
	return available
}
