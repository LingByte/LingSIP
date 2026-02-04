package config

import (
	"os"
	"testing"
)

func TestConfigLoad(t *testing.T) {
	// 保存原始GlobalConfig
	originalGlobalConfig := GlobalConfig
	defer func() {
		GlobalConfig = originalGlobalConfig
	}()

	// 设置测试环境变量
	os.Setenv("LLM_PROVIDER", "test-llm")
	os.Setenv("LLM_API_KEY", "test-key")
	os.Setenv("ASR_PROVIDER", "test-asr")
	os.Setenv("TTS_PROVIDER", "test-tts")

	defer func() {
		os.Unsetenv("LLM_PROVIDER")
		os.Unsetenv("LLM_API_KEY")
		os.Unsetenv("ASR_PROVIDER")
		os.Unsetenv("TTS_PROVIDER")
	}()

	err := Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if GlobalConfig == nil {
		t.Fatal("GlobalConfig is nil")
	}

	// 测试LLM配置
	if GlobalConfig.Services.LLM.Provider != "test-llm" {
		t.Errorf("Expected LLM provider 'test-llm', got '%s'", GlobalConfig.Services.LLM.Provider)
	}

	if GlobalConfig.Services.LLM.APIKey != "test-key" {
		t.Errorf("Expected LLM API key 'test-key', got '%s'", GlobalConfig.Services.LLM.APIKey)
	}

	// 测试ASR配置
	if GlobalConfig.Services.ASR.Provider != "test-asr" {
		t.Errorf("Expected ASR provider 'test-asr', got '%s'", GlobalConfig.Services.ASR.Provider)
	}

	// 测试TTS配置
	if GlobalConfig.Services.TTS.Provider != "test-tts" {
		t.Errorf("Expected TTS provider 'test-tts', got '%s'", GlobalConfig.Services.TTS.Provider)
	}
}

func TestConfigStructure(t *testing.T) {
	// 保存原始GlobalConfig
	originalGlobalConfig := GlobalConfig
	defer func() {
		GlobalConfig = originalGlobalConfig
	}()

	err := Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if GlobalConfig == nil {
		t.Fatal("GlobalConfig is nil")
	}

	// 测试配置结构是否完整
	if GlobalConfig.Services.LLM.Provider == "" {
		t.Error("LLM provider should not be empty")
	}

	if GlobalConfig.Services.ASR.Provider == "" {
		t.Error("ASR provider should not be empty")
	}

	if GlobalConfig.Services.TTS.Provider == "" {
		t.Error("TTS provider should not be empty")
	}

	// 测试默认值是否合理
	if GlobalConfig.Services.LLM.Temperature <= 0 || GlobalConfig.Services.LLM.Temperature > 2 {
		t.Errorf("LLM temperature should be between 0 and 2, got %f", GlobalConfig.Services.LLM.Temperature)
	}

	if GlobalConfig.Services.LLM.MaxTokens <= 0 {
		t.Errorf("LLM max tokens should be positive, got %d", GlobalConfig.Services.LLM.MaxTokens)
	}

	if GlobalConfig.Services.TTS.SampleRate <= 0 {
		t.Errorf("TTS sample rate should be positive, got %d", GlobalConfig.Services.TTS.SampleRate)
	}
}

func TestConfigValidation(t *testing.T) {
	// 保存原始GlobalConfig
	originalGlobalConfig := GlobalConfig
	defer func() {
		GlobalConfig = originalGlobalConfig
	}()

	// 设置最小必需配置
	os.Setenv("DSN", "test.db")
	os.Setenv("ADDR", ":8080")

	defer func() {
		os.Unsetenv("DSN")
		os.Unsetenv("ADDR")
	}()

	err := Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	err = GlobalConfig.Validate()
	if err != nil {
		t.Errorf("Config validation failed: %v", err)
	}
}
