package llm

import (
	"context"
	"fmt"

	"github.com/LingByte/LingSIP/pkg/config"
	"github.com/sirupsen/logrus"
)

// Config holds the configuration for LLM service
type Config struct {
	Provider     string  `json:"provider" yaml:"provider"`
	APIKey       string  `json:"api_key" yaml:"api_key"`
	BaseURL      string  `json:"base_url" yaml:"base_url"`
	Model        string  `json:"model" yaml:"model"`
	Temperature  float32 `json:"temperature" yaml:"temperature"`
	MaxTokens    int     `json:"max_tokens" yaml:"max_tokens"`
	StreamingTTS bool    `json:"streaming_tts" yaml:"streaming_tts"`
}

// DefaultConfig returns a default configuration using global config
func DefaultConfig() *Config {
	if config.GlobalConfig == nil {
		// Fallback to hardcoded values if global config is not available
		return &Config{
			Provider:     "qwen",
			APIKey:       "sk-1b7618ac4d9343f3b5aefcd74f4cf428",
			BaseURL:      "https://dashscope.aliyuncs.com/compatible-mode/v1",
			Model:        "qwen-plus",
			Temperature:  0.7,
			MaxTokens:    2000,
			StreamingTTS: false,
		}
	}

	llmConfig := config.GlobalConfig.Services.LLM
	return &Config{
		Provider:     llmConfig.Provider,
		APIKey:       llmConfig.APIKey,
		BaseURL:      llmConfig.BaseURL,
		Model:        llmConfig.Model,
		Temperature:  llmConfig.Temperature,
		MaxTokens:    llmConfig.MaxTokens,
		StreamingTTS: false,
	}
}

// Service represents the LLM service
type Service struct {
	config  *Config
	handler *LLMHandler
	logger  *logrus.Logger
}

// NewService creates a new LLM service
func NewService(config *Config, logger *logrus.Logger) *Service {
	if config == nil {
		config = DefaultConfig()
	}

	return &Service{
		config: config,
		logger: logger,
	}
}

// Initialize initializes the LLM service with a system prompt
func (s *Service) Initialize(ctx context.Context, systemPrompt string) error {
	if s.config.APIKey == "" {
		return fmt.Errorf("API key is required")
	}

	if s.config.BaseURL == "" {
		return fmt.Errorf("base URL is required")
	}

	s.handler = NewLLMHandler(ctx, s.config.APIKey, s.config.BaseURL, systemPrompt, s.logger)

	s.logger.WithFields(logrus.Fields{
		"provider": s.config.Provider,
		"base_url": s.config.BaseURL,
		"model":    s.config.Model,
	}).Info("LLM service initialized")

	return nil
}

// GetHandler returns the LLM handler
func (s *Service) GetHandler() *LLMHandler {
	return s.handler
}

// Query performs a simple query to the LLM
func (s *Service) Query(text string) (string, error) {
	if s.handler == nil {
		return "", fmt.Errorf("LLM service not initialized")
	}

	return s.handler.Query(s.config.Model, text)
}

// QueryStream performs a streaming query to the LLM
func (s *Service) QueryStream(text string, client TTSClient, referCaller string) (string, error) {
	if s.handler == nil {
		return "", fmt.Errorf("LLM service not initialized")
	}

	return s.handler.QueryStream(s.config.Model, text, s.config.StreamingTTS, client, referCaller)
}

// Reset resets the conversation history
func (s *Service) Reset() {
	if s.handler != nil {
		s.handler.Reset()
	}
}

// UpdateConfig updates the service configuration
func (s *Service) UpdateConfig(config *Config) {
	s.config = config
}

// GetConfig returns the current configuration
func (s *Service) GetConfig() *Config {
	return s.config
}
