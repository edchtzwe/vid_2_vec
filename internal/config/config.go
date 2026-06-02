package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DatabaseURL string
	DBSchema    string

	DBMigrationSchema string
	DBMigrationTable  string

	RedisURL string

	AppPort string
	AppHost string

	AIProvider              string
	GoogleAIKey             string
	GoogleAIGenerativeModel string
	GoogleAIEmbeddingModel  string
	GoogleAIFileBaseURI     string
	AIAPIMaxRetries         int
	AIAPIRetryAfterDelay    time.Duration
	GeminiAnalysisPrompt    string

	SourceFileDir string

	UploaderLogPath     string
	StateCheckerLogPath string
	AnalyzerLogPath     string
	EmbedderLogPath     string
	AnalyzerOutputDir   string
}

func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		DBSchema:    envOrDefault("DB_SCHEMA", "public"),

		DBMigrationSchema: envOrDefault("DB_MIGRATION_SCHEMA", "public"),
		DBMigrationTable:  envOrDefault("DB_MIGRATION_TABLE", "schema_migrations_go"),

		RedisURL: envOrDefault("REDIS_URL", "redis://127.0.0.1:6379/0"),

		AppPort: envOrDefault("APP_PORT", "8080"),
		AppHost: envOrDefault("APP_HOST", "0.0.0.0"),

		AIProvider:              envOrDefault("AI_PROVIDER", "google"),
		GoogleAIKey:             os.Getenv("GOOGLE_AI_KEY"),
		GoogleAIGenerativeModel: envOrDefault("GOOGLE_AI_GENERATIVE_MODEL", "gemini-2.5-flash"),
		GoogleAIEmbeddingModel:  envOrDefault("GOOGLE_AI_EMBEDDING_MODEL", "gemini-embedding-001"),
		GoogleAIFileBaseURI:     envOrDefault("GOOGLE_AI_FILE_BASE_URI", "https://generativelanguage.googleapis.com/v1beta/"),
		GeminiAnalysisPrompt:    os.Getenv("GEMINI_ANALYSIS_PROMPT"),

		SourceFileDir: envOrDefault("SOURCE_FILE_DIR", "/app/data/source"),

		UploaderLogPath:     envOrDefault("UPLOADER_LOG_PATH", "./logs/uploader.log"),
		StateCheckerLogPath: envOrDefault("STATE_CHECKER_LOG_PATH", "./logs/state_checker.log"),
		AnalyzerLogPath:     envOrDefault("ANALYZER_LOG_PATH", "./logs/analyzer.log"),
		EmbedderLogPath:     envOrDefault("EMBEDDER_LOG_PATH", "./logs/embedder.log"),
		AnalyzerOutputDir:   envOrDefault("ANALYZER_OUTPUT_DIR", "./manifests"),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.GoogleAIKey == "" {
		return nil, fmt.Errorf("GOOGLE_AI_KEY is required")
	}
	if cfg.GeminiAnalysisPrompt == "" {
		return nil, fmt.Errorf("GEMINI_ANALYSIS_PROMPT is required")
	}

	retries, _ := strconv.Atoi(envOrDefault("AI_API_MAX_RETRIES", "3"))
	cfg.AIAPIMaxRetries = retries

	delayMs, _ := strconv.Atoi(envOrDefault("AI_API_RETRY_AFTER_DELAY", "1000"))
	cfg.AIAPIRetryAfterDelay = time.Duration(delayMs) * time.Millisecond

	return cfg, nil
}

func (c *Config) ListenAddr() string {
	return c.AppHost + ":" + c.AppPort
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}