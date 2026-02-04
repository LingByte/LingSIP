package config

import (
	"errors"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/LingByte/LingSIP/pkg/logger"
	"github.com/LingByte/LingSIP/pkg/notification"
	"github.com/LingByte/LingSIP/pkg/utils"
)

// Config main configuration structure
type Config struct {
	MachineID  int64            `env:"MACHINE_ID"`
	Server     ServerConfig     `mapstructure:"server"`
	Database   DatabaseConfig   `mapstructure:"database"`
	Log        logger.LogConfig `mapstructure:"log"`
	Services   ServicesConfig   `mapstructure:"services"`
	Middleware MiddlewareConfig `mapstructure:"middleware"`
}

// ServerConfig server configuration
type ServerConfig struct {
	Name          string `env:"SERVER_NAME"`
	Desc          string `env:"SERVER_DESC"`
	URL           string `env:"SERVER_URL"`
	Logo          string `env:"SERVER_LOGO"`
	TermsURL      string `env:"SERVER_TERMS_URL"`
	Addr          string `env:"ADDR"`
	Mode          string `env:"MODE"`
	DocsPrefix    string `env:"DOCS_PREFIX"`
	APIPrefix     string `env:"API_PREFIX"`
	AdminPrefix   string `env:"ADMIN_PREFIX"`
	AuthPrefix    string `env:"AUTH_PREFIX"`
	MonitorPrefix string `env:"MONITOR_PREFIX"`
	SSLEnabled    bool   `env:"SSL_ENABLED"`
	SSLCertFile   string `env:"SSL_CERT_FILE"`
	SSLKeyFile    string `env:"SSL_KEY_FILE"`
}

// DatabaseConfig database configuration
type DatabaseConfig struct {
	Driver string `env:"DB_DRIVER"`
	DSN    string `env:"DSN"`
}

// ServicesConfig services configuration
type ServicesConfig struct {
	LLM  LLMConfig               `mapstructure:"llm"`
	ASR  ASRConfig               `mapstructure:"asr"`
	TTS  TTSConfig               `mapstructure:"tts"`
	Mail notification.MailConfig `mapstructure:"mail"`
}

// LLMConfig LLM service configuration
type LLMConfig struct {
	Provider    string  `env:"LLM_PROVIDER"` // openai, qwen, etc.
	APIKey      string  `env:"LLM_API_KEY"`
	BaseURL     string  `env:"LLM_BASE_URL"`
	Model       string  `env:"LLM_MODEL"`
	Temperature float32 `env:"LLM_TEMPERATURE"`
	MaxTokens   int     `env:"LLM_MAX_TOKENS"`
}

// ASRConfig ASR service configuration
type ASRConfig struct {
	Provider  string `env:"ASR_PROVIDER"` // qcloud, baidu, aws, etc.
	AppID     string `env:"ASR_APP_ID"`
	SecretID  string `env:"ASR_SECRET_ID"`
	SecretKey string `env:"ASR_SECRET_KEY"`
	Region    string `env:"ASR_REGION"`
	ModelType string `env:"ASR_MODEL_TYPE"` // 8k_zh, 16k_zh, etc.
	Language  string `env:"ASR_LANGUAGE"`   // zh-CN, en-US, etc.
}

// TTSConfig TTS service configuration
type TTSConfig struct {
	Provider   string `env:"TTS_PROVIDER"` // qcloud, baidu, aws, etc.
	AppID      string `env:"TTS_APP_ID"`
	SecretID   string `env:"TTS_SECRET_ID"`
	SecretKey  string `env:"TTS_SECRET_KEY"`
	Region     string `env:"TTS_REGION"`
	VoiceType  string `env:"TTS_VOICE_TYPE"`  // 601002, etc.
	SampleRate int    `env:"TTS_SAMPLE_RATE"` // 8000, 16000, etc.
	Codec      string `env:"TTS_CODEC"`       // pcm, mp3, etc.
	Language   string `env:"TTS_LANGUAGE"`    // zh-CN, en-US, etc.
}

// MiddlewareConfig middleware configuration
type MiddlewareConfig struct {
	// Rate limiting configuration
	RateLimit RateLimiterConfig
	// Timeout configuration
	Timeout TimeoutConfig
	// Circuit breaker configuration
	CircuitBreaker CircuitBreakerConfig
	// Whether to enable each middleware
	EnableRateLimit      bool `env:"ENABLE_RATE_LIMIT"`
	EnableTimeout        bool `env:"ENABLE_TIMEOUT"`
	EnableCircuitBreaker bool `env:"ENABLE_CIRCUIT_BREAKER"`
	EnableOperationLog   bool `env:"ENABLE_OPERATION_LOG"`
}

// RateLimiterConfig rate limiting configuration
type RateLimiterConfig struct {
	GlobalRPS    int           `env:"RATE_LIMIT_GLOBAL_RPS"`   // Global requests per second
	GlobalBurst  int           `env:"RATE_LIMIT_GLOBAL_BURST"` // Global burst requests
	GlobalWindow time.Duration // Global time window
	UserRPS      int           `env:"RATE_LIMIT_USER_RPS"`   // User requests per second
	UserBurst    int           `env:"RATE_LIMIT_USER_BURST"` // User burst requests
	UserWindow   time.Duration // User time window
	IPRPS        int           `env:"RATE_LIMIT_IP_RPS"`   // IP requests per second
	IPBurst      int           `env:"RATE_LIMIT_IP_BURST"` // IP burst requests
	IPWindow     time.Duration // IP time window
}

// TimeoutConfig timeout configuration
type TimeoutConfig struct {
	DefaultTimeout   time.Duration `env:"DEFAULT_TIMEOUT"`
	FallbackResponse interface{}
}

// CircuitBreakerConfig circuit breaker configuration
type CircuitBreakerConfig struct {
	FailureThreshold      int           `env:"CIRCUIT_BREAKER_FAILURE_THRESHOLD"`
	SuccessThreshold      int           `env:"CIRCUIT_BREAKER_SUCCESS_THRESHOLD"`
	Timeout               time.Duration `env:"CIRCUIT_BREAKER_TIMEOUT"`
	OpenTimeout           time.Duration `env:"CIRCUIT_BREAKER_OPEN_TIMEOUT"`
	MaxConcurrentRequests int           `env:"CIRCUIT_BREAKER_MAX_CONCURRENT"`
}

var GlobalConfig *Config

func Load() error {
	// 1. Load .env file based on environment (don't error if it doesn't exist, use default values)
	env := os.Getenv("APP_ENV")
	err := utils.LoadEnv(env)
	if err != nil {
		// Only log when .env file doesn't exist, don't affect startup
		log.Printf("Note: .env file not found or failed to load: %v (using default values)", err)
	}

	// 2. Load global configuration
	GlobalConfig = &Config{
		MachineID: utils.GetIntEnv("MACHINE_ID"),
		Server: ServerConfig{
			Name:          getStringOrDefault("SERVER_NAME", ""),
			Desc:          getStringOrDefault("SERVER_DESC", ""),
			URL:           getStringOrDefault("SERVER_URL", ""),
			Logo:          getStringOrDefault("SERVER_LOGO", ""),
			TermsURL:      getStringOrDefault("SERVER_TERMS_URL", ""),
			Addr:          getStringOrDefault("ADDR", ":7072"),
			Mode:          getStringOrDefault("MODE", "development"),
			DocsPrefix:    getStringOrDefault("DOCS_PREFIX", "/api/docs"),
			APIPrefix:     getStringOrDefault("API_PREFIX", "/api"),
			AdminPrefix:   getStringOrDefault("ADMIN_PREFIX", "/admin"),
			AuthPrefix:    getStringOrDefault("AUTH_PREFIX", "/auth"),
			MonitorPrefix: getStringOrDefault("MONITOR_PREFIX", "/metrics"),
			SSLEnabled:    getBoolOrDefault("SSL_ENABLED", false),
			SSLCertFile:   getStringOrDefault("SSL_CERT_FILE", ""),
			SSLKeyFile:    getStringOrDefault("SSL_KEY_FILE", ""),
		},
		Database: DatabaseConfig{
			Driver: getStringOrDefault("DB_DRIVER", "sqlite"),
			DSN:    getStringOrDefault("DSN", "./ling.db"),
		},
		Log: logger.LogConfig{
			Level:      getStringOrDefault("LOG_LEVEL", "info"),
			Filename:   getStringOrDefault("LOG_FILENAME", "./logs/app.log"),
			MaxSize:    getIntOrDefault("LOG_MAX_SIZE", 100),
			MaxAge:     getIntOrDefault("LOG_MAX_AGE", 30),
			MaxBackups: getIntOrDefault("LOG_MAX_BACKUPS", 5),
			Daily:      getBoolOrDefault("LOG_DAILY", true),
		},
		Services: ServicesConfig{
			LLM: LLMConfig{
				Provider:    getStringOrDefault("LLM_PROVIDER", "openai"),
				APIKey:      getStringOrDefault("LLM_API_KEY", ""),
				BaseURL:     getStringOrDefault("LLM_BASE_URL", "https://api.openai.com/v1"),
				Model:       getStringOrDefault("LLM_MODEL", "gpt-3.5-turbo"),
				Temperature: float32(getFloatOrDefault("LLM_TEMPERATURE", 0.7)),
				MaxTokens:   getIntOrDefault("LLM_MAX_TOKENS", 2000),
			},
			ASR: ASRConfig{
				Provider:  getStringOrDefault("ASR_PROVIDER", "qcloud"),
				AppID:     getStringOrDefault("ASR_APP_ID", ""),
				SecretID:  getStringOrDefault("ASR_SECRET_ID", ""),
				SecretKey: getStringOrDefault("ASR_SECRET_KEY", ""),
				Region:    getStringOrDefault("ASR_REGION", "ap-beijing"),
				ModelType: getStringOrDefault("ASR_MODEL_TYPE", "8k_zh"),
				Language:  getStringOrDefault("ASR_LANGUAGE", "zh-CN"),
			},
			TTS: TTSConfig{
				Provider:   getStringOrDefault("TTS_PROVIDER", "qcloud"),
				AppID:      getStringOrDefault("TTS_APP_ID", ""),
				SecretID:   getStringOrDefault("TTS_SECRET_ID", ""),
				SecretKey:  getStringOrDefault("TTS_SECRET_KEY", ""),
				Region:     getStringOrDefault("TTS_REGION", "ap-beijing"),
				VoiceType:  getStringOrDefault("TTS_VOICE_TYPE", "601002"),
				SampleRate: getIntOrDefault("TTS_SAMPLE_RATE", 8000),
				Codec:      getStringOrDefault("TTS_CODEC", "pcm"),
				Language:   getStringOrDefault("TTS_LANGUAGE", "zh-CN"),
			},
			Mail: notification.MailConfig{
				Host:     getStringOrDefault("MAIL_HOST", ""),
				Username: getStringOrDefault("MAIL_USERNAME", ""),
				Password: getStringOrDefault("MAIL_PASSWORD", ""),
				Port:     int64(getIntOrDefault("MAIL_PORT", 587)),
				From:     getStringOrDefault("MAIL_FROM", ""),
			},
		},
		Middleware: loadMiddlewareConfig(),
	}
	return nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate database configuration
	if c.Database.DSN == "" {
		return errors.New("database DSN is required")
	}

	// Validate server configuration
	if c.Server.Addr == "" {
		return errors.New("server address is required")
	}
	return nil
}

// getStringOrDefault gets environment variable value, returns default if empty
func getStringOrDefault(key, defaultValue string) string {
	value := utils.GetEnv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// getBoolOrDefault gets boolean environment variable value, returns default if empty
func getBoolOrDefault(key string, defaultValue bool) bool {
	value := utils.GetEnv(key)
	if value == "" {
		return defaultValue
	}
	return utils.GetBoolEnv(key)
}

// getIntOrDefault gets integer environment variable value, returns default if empty
func getIntOrDefault(key string, defaultValue int) int {
	value := utils.GetIntEnv(key)
	if value == 0 {
		return defaultValue
	}
	return int(value)
}

// getFloatOrDefault gets float environment variable value, returns default if empty
func getFloatOrDefault(key string, defaultValue float64) float64 {
	value := utils.GetEnv(key)
	if value == "" {
		return defaultValue
	}
	// 简单的字符串到float64转换
	if f, err := strconv.ParseFloat(value, 64); err == nil {
		return f
	}
	return defaultValue
}

// parseDuration parses duration string with default fallback
func parseDuration(s string, defaultVal time.Duration) time.Duration {
	if s == "" {
		return defaultVal
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return defaultVal
	}
	return d
}

// generateDefaultSessionSecret generates default session secret (for development only)
func generateDefaultSessionSecret() string {
	if secret := utils.GetEnv("SESSION_SECRET"); secret != "" {
		return secret
	}
	return "default-secret-key-change-in-production-" + utils.RandText(16)
}

// loadMiddlewareConfig loads middleware configuration
func loadMiddlewareConfig() MiddlewareConfig {
	mode := getStringOrDefault("MODE", "development")
	var defaultConfig MiddlewareConfig

	if mode == "production" {
		defaultConfig = MiddlewareConfig{
			RateLimit: RateLimiterConfig{
				GlobalRPS:    2000,
				GlobalBurst:  4000,
				GlobalWindow: time.Minute,
				UserRPS:      200,
				UserBurst:    400,
				UserWindow:   time.Minute,
				IPRPS:        100,
				IPBurst:      200,
				IPWindow:     time.Minute,
			},
			Timeout: TimeoutConfig{
				DefaultTimeout: 30 * time.Second,
				FallbackResponse: map[string]interface{}{
					"error":   "service_unavailable",
					"message": "Service temporarily unavailable, please try again later",
					"code":    503,
				},
			},
			CircuitBreaker: CircuitBreakerConfig{
				FailureThreshold:      3,
				SuccessThreshold:      2,
				Timeout:               30 * time.Second,
				OpenTimeout:           30 * time.Second,
				MaxConcurrentRequests: 200,
			},
			EnableRateLimit:      true,
			EnableTimeout:        true,
			EnableCircuitBreaker: true,
			EnableOperationLog:   true,
		}
	} else {
		defaultConfig = MiddlewareConfig{
			RateLimit: RateLimiterConfig{
				GlobalRPS:    10000,
				GlobalBurst:  20000,
				GlobalWindow: time.Minute,
				UserRPS:      1000,
				UserBurst:    2000,
				UserWindow:   time.Minute,
				IPRPS:        500,
				IPBurst:      1000,
				IPWindow:     time.Minute,
			},
			Timeout: TimeoutConfig{
				DefaultTimeout: 60 * time.Second,
				FallbackResponse: map[string]interface{}{
					"error":   "service_unavailable",
					"message": "Service temporarily unavailable, please try again later",
					"code":    503,
				},
			},
			CircuitBreaker: CircuitBreakerConfig{
				FailureThreshold:      10,
				SuccessThreshold:      5,
				Timeout:               60 * time.Second,
				OpenTimeout:           60 * time.Second,
				MaxConcurrentRequests: 1000,
			},
			EnableRateLimit:      true,
			EnableTimeout:        true,
			EnableCircuitBreaker: false,
			EnableOperationLog:   true,
		}
	}
	return MiddlewareConfig{
		RateLimit: RateLimiterConfig{
			GlobalRPS:    getIntOrDefault("RATE_LIMIT_GLOBAL_RPS", defaultConfig.RateLimit.GlobalRPS),
			GlobalBurst:  getIntOrDefault("RATE_LIMIT_GLOBAL_BURST", defaultConfig.RateLimit.GlobalBurst),
			GlobalWindow: parseDuration(getStringOrDefault("RATE_LIMIT_GLOBAL_WINDOW", "1m"), defaultConfig.RateLimit.GlobalWindow),
			UserRPS:      getIntOrDefault("RATE_LIMIT_USER_RPS", defaultConfig.RateLimit.UserRPS),
			UserBurst:    getIntOrDefault("RATE_LIMIT_USER_BURST", defaultConfig.RateLimit.UserBurst),
			UserWindow:   parseDuration(getStringOrDefault("RATE_LIMIT_USER_WINDOW", "1m"), defaultConfig.RateLimit.UserWindow),
			IPRPS:        getIntOrDefault("RATE_LIMIT_IP_RPS", defaultConfig.RateLimit.IPRPS),
			IPBurst:      getIntOrDefault("RATE_LIMIT_IP_BURST", defaultConfig.RateLimit.IPBurst),
			IPWindow:     parseDuration(getStringOrDefault("RATE_LIMIT_IP_WINDOW", "1m"), defaultConfig.RateLimit.IPWindow),
		},
		Timeout: TimeoutConfig{
			DefaultTimeout:   parseDuration(getStringOrDefault("DEFAULT_TIMEOUT", "30s"), defaultConfig.Timeout.DefaultTimeout),
			FallbackResponse: defaultConfig.Timeout.FallbackResponse,
		},
		CircuitBreaker: CircuitBreakerConfig{
			FailureThreshold:      getIntOrDefault("CIRCUIT_BREAKER_FAILURE_THRESHOLD", defaultConfig.CircuitBreaker.FailureThreshold),
			SuccessThreshold:      getIntOrDefault("CIRCUIT_BREAKER_SUCCESS_THRESHOLD", defaultConfig.CircuitBreaker.SuccessThreshold),
			Timeout:               parseDuration(getStringOrDefault("CIRCUIT_BREAKER_TIMEOUT", "30s"), defaultConfig.CircuitBreaker.Timeout),
			OpenTimeout:           parseDuration(getStringOrDefault("CIRCUIT_BREAKER_OPEN_TIMEOUT", "30s"), defaultConfig.CircuitBreaker.OpenTimeout),
			MaxConcurrentRequests: getIntOrDefault("CIRCUIT_BREAKER_MAX_CONCURRENT", defaultConfig.CircuitBreaker.MaxConcurrentRequests),
		},
		EnableRateLimit:      getBoolOrDefault("ENABLE_RATE_LIMIT", defaultConfig.EnableRateLimit),
		EnableTimeout:        getBoolOrDefault("ENABLE_TIMEOUT", defaultConfig.EnableTimeout),
		EnableCircuitBreaker: getBoolOrDefault("ENABLE_CIRCUIT_BREAKER", defaultConfig.EnableCircuitBreaker),
		EnableOperationLog:   getBoolOrDefault("ENABLE_OPERATION_LOG", defaultConfig.EnableOperationLog),
	}
}
