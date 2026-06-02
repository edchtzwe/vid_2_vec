package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time" // CHANGE: Added time for retry backoff

	"ingestion/internal/config"

	"google.golang.org/genai"
)

// ─── Data Structures ─────────────────────────────────────────────────────────

// FileInfo contains the upload result needed by the Service/Repo.
type FileInfo struct {
	URI  string // The "files/..." ID used for referencing in prompts
	Name string // The display name or resource name
}

// GeminiMetadata is the canonical output written to manifests.
type GeminiMetadata struct {
	VideoTitle     string          `json:"video_title"`
	VideoCategory  string          `json:"video_category"`
	GlobalKeywords[]string        `json:"global_keywords"`
	Duration       float64         `json:"duration"`
	Scenes[]SceneMetadata `json:"scenes"`
}

// SceneMetadata represents a single scene in the canonical format.
type SceneMetadata struct {
	StartTime float64  `json:"start_time"`
	EndTime   float64  `json:"end_time"`
	Name      string   `json:"name"`
	Content   string   `json:"content"`
	Items[]string `json:"items"`
}

// responseSchema defines the exact JSON shape Gemini must return.
// This is enforced at generation time via structured outputs.
var responseSchema = &genai.Schema{
	Type:        genai.TypeObject,
	Description: "Video analysis with video-level context and per-scene breakdowns",
	Properties: map[string]*genai.Schema{
		"video_title": {
			Type:        genai.TypeString,
			Description: "A specific, distinctive title for this video that distinguishes it from similar content (e.g. \"Mama Lin's Garlic Green Beans\" not \"Cooking Tutorial\")",
		},
		"video_category": {
			Type:        genai.TypeString,
			Description: "High-level genre or category (e.g. \"Cooking Tutorial\", \"Product Review\", \"Travel Vlog\")",
		},
		"global_keywords": {
			Type:        genai.TypeArray,
			Description: "3-5 distinctive tags that characterize the whole video (e.g. \"Vegetarian\", \"Wok\", \"Side Dish\")",
			Items:       &genai.Schema{Type: genai.TypeString},
		},
		"duration": {
			Type:        genai.TypeNumber,
			Description: "Total video duration in SECONDS as a plain number (e.g. 350 for 5m50s). Do NOT use MM:SS. Must be > 0",
		},
		"scenes": {
			Type:        genai.TypeArray,
			Description: "Array of scene breakdowns covering the full video",
			Items: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"start_time": {
						Type:        genai.TypeNumber,
						Description: "When the scene begins, in SECONDS as a plain number (e.g. 90 for 1m30s, NOT 1.30). Must be >= 0",
					},
					"end_time": {
						Type:        genai.TypeNumber,
						Description: "When the scene ends, in SECONDS as a plain number (e.g. 150 for 2m30s, NOT 2.30). Must be > start_time and <= duration",
					},
					"name": {
						Type:        genai.TypeString,
						Description: "Short descriptive title for the scene (2-6 words)",
					},
				"content": {
					Type:        genai.TypeString,
					Description: "Factual description of ONLY the visual actions, subjects, settings, and any on-screen text or dialogue visible in this scene (at least one full sentence). Do NOT include the video title, category, or keywords — those are captured in the video-level fields.",
				},
					"items": {
						Type:        genai.TypeArray,
						Description: "Specific objects, ingredients, tools, or products visible in the scene",
						Items:       &genai.Schema{Type: genai.TypeString},
					},
				},
				PropertyOrdering:[]string{"start_time", "end_time", "name", "content", "items"},
				Required:[]string{"start_time", "end_time", "name", "content", "items"},
			},
		},
	},
	PropertyOrdering:[]string{"video_title", "video_category", "global_keywords", "duration", "scenes"},
	Required:[]string{"video_title", "video_category", "global_keywords", "duration", "scenes"},
}

// ─── Client Definition ───────────────────────────────────────────────────────

type Client struct {
	apiKey              string
	generativeModel     string
	embeddingModel      string
	fileBaseURI         string
	maxAnalysisAttempts int
	analysisPrompt      string
	logger              *log.Logger
}

func NewClient(cfg *config.Config, logger *log.Logger) *Client {
	attempts := cfg.AIAPIMaxRetries
	if attempts <= 0 {
		attempts = 3
	}
	if logger == nil {
		logger = log.Default()
	}
	return &Client{
		apiKey:              cfg.GoogleAIKey,
		generativeModel:     cfg.GoogleAIGenerativeModel,
		embeddingModel:      cfg.GoogleAIEmbeddingModel,
		fileBaseURI:         cfg.GoogleAIFileBaseURI,
		maxAnalysisAttempts: attempts,
		analysisPrompt:      cfg.GeminiAnalysisPrompt,
		logger:              logger,
	}
}

// ─── Methods ─────────────────────────────────────────────────────────────────

// mimeForVideo returns the correct MIME type for video files, falling back to
// extension-based detection for non-video paths.
func mimeForVideo(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp4":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".avi":
		return "video/x-msvideo"
	case ".webm":
		return "video/webm"
	case ".mkv":
		return "video/x-matroska"
	default:
		return "application/octet-stream"
	}
}

// UploadFile uploads the file to Gemini using the new SDK.
// It returns immediately after upload; it does NOT wait for processing.
func (c *Client) UploadFile(ctx context.Context, path string) (*FileInfo, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  c.apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create genai client: %w", err)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	mimeType := mimeForVideo(path)
	c.logger.Printf("UploadFile: path=%s mimeType=%s", path, mimeType)

	resp, err := client.Files.Upload(ctx, f, &genai.UploadFileConfig{
		MIMEType: mimeType,
	})
	if err != nil {
		return nil, fmt.Errorf("google api upload failed (mime=%s): %w", mimeType, err)
	}

	return &FileInfo{
		URI:  resp.URI,
		Name: resp.Name,
	}, nil
}

// GetFileState checks the processing status of a file.
func (c *Client) GetFileState(ctx context.Context, name string) (string, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  c.apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create genai client: %w", err)
	}

	file, err := client.Files.Get(ctx, name, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get file state: %w", err)
	}

	return string(file.State), nil
}

// GenerateMetadata analyzes the video and returns structured scenes.
// It retries up to c.maxAnalysisAttempts times if the model returns incomplete data.
func (c *Client) GenerateMetadata(ctx context.Context, fileName string) (*GeminiMetadata, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  c.apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create genai client: %w", err)
	}

	fileObj, err := client.Files.Get(ctx, fileName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve file uri for %s: %w", fileName, err)
	}

	c.logger.Printf("GenerateMetadata: model=%s file=%s uri=%s mime=%s",
		c.generativeModel, fileName, fileObj.URI, fileObj.MIMEType)

	fileDataPart := &genai.Part{
		FileData: &genai.FileData{
			MIMEType: fileObj.MIMEType,
			FileURI:  fileObj.URI,
		},
	}

	history := []*genai.Content{
		{Role: "user", Parts:[]*genai.Part{{Text: c.analysisPrompt}, fileDataPart}},
	}

	var lastErr error
	var lastMetadata *GeminiMetadata
	for attempt := 1; attempt <= c.maxAnalysisAttempts; attempt++ {
		c.logger.Printf("GenerateMetadata attempt %d/%d for %s", attempt, c.maxAnalysisAttempts, fileName)

		resp, err := client.Models.GenerateContent(ctx, c.generativeModel,
			history,
			&genai.GenerateContentConfig{
				ResponseMIMEType:   "application/json",
				ResponseSchema:     responseSchema,
			},
		)
		if err != nil {
			c.dumpAPIError(fmt.Sprintf("GenerateContent attempt %d (model=%s, file=%s)", attempt, c.generativeModel, fileName), err)
			lastErr = fmt.Errorf("generate content failed (attempt %d): %w", attempt, err)
			c.logger.Printf("GenerateMetadata attempt %d/%d failed for %s: %v (retrying)", attempt, c.maxAnalysisAttempts, fileName, lastErr)
			continue
		}

		if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
			lastErr = fmt.Errorf("empty response from gemini (attempt %d, no candidates)", attempt)
			c.logger.Printf("GenerateMetadata attempt %d/%d failed for %s: %v (retrying)", attempt, c.maxAnalysisAttempts, fileName, lastErr)
			continue
		}

		rawText := resp.Candidates[0].Content.Parts[0].Text

		var metadata GeminiMetadata
		if err := json.Unmarshal([]byte(rawText), &metadata); err != nil {
			lastErr = fmt.Errorf("unmarshal failed (attempt %d): %w\nraw:\n%s", attempt, err, rawText)
			c.logger.Printf("GenerateMetadata attempt %d/%d failed for %s: %v (retrying)", attempt, c.maxAnalysisAttempts, fileName, lastErr)
			continue
		}

		lastMetadata = &metadata

		if problems := validateMetadata(&metadata); len(problems) > 0 {
			feedback := fmt.Sprintf(
				"Your previous response had the following problems:\n%s\n\nPlease return the complete JSON again with ALL fields populated. Every scene must have a non-empty \"name\" and a non-empty \"content\".",
				strings.Join(problems, "\n"),
			)
			lastErr = fmt.Errorf("validation failed (attempt %d): %s", attempt, strings.Join(problems, "; "))
			c.logger.Printf("GenerateMetadata attempt %d/%d validation failed for %s:\n%s", attempt, c.maxAnalysisAttempts, fileName, strings.Join(problems, "\n"))

			history = append(history,
				&genai.Content{Role: "model", Parts:[]*genai.Part{{Text: rawText}}},
				&genai.Content{Role: "user", Parts:[]*genai.Part{{Text: feedback}}},
			)
			continue
		}

		c.logger.Printf("GenerateMetadata succeeded on attempt %d/%d for %s", attempt, c.maxAnalysisAttempts, fileName)
		return &metadata, nil
	}

	c.logger.Printf("GenerateMetadata exhausted all %d attempts for %s: %v", c.maxAnalysisAttempts, fileName, lastErr)
	return lastMetadata, fmt.Errorf("all %d attempts failed: %w", c.maxAnalysisAttempts, lastErr)
}

// validateMetadata performs strict validation on the deserialized response,
// catching semantic issues that the JSON schema alone cannot enforce
// (e.g. zero durations, empty strings, nonsensical timestamps).
func validateMetadata(m *GeminiMetadata) []string {
	var problems[]string

	if strings.TrimSpace(m.VideoTitle) == "" {
		problems = append(problems, "- \"video_title\" is missing or empty")
	}
	if strings.TrimSpace(m.VideoCategory) == "" {
		problems = append(problems, "- \"video_category\" is missing or empty")
	}
	if len(m.GlobalKeywords) == 0 {
		problems = append(problems, "- \"global_keywords\" is empty (need 3-5 tags)")
	}
	if m.Duration <= 0 {
		problems = append(problems, "- \"duration\" is missing or zero")
	}
	if len(m.Scenes) == 0 {
		problems = append(problems, "- \"scenes\" array is empty")
		return problems
	}

	for i, s := range m.Scenes {
		if strings.TrimSpace(s.Name) == "" {
			problems = append(problems, fmt.Sprintf("- scene[%d]: \"name\" is empty", i))
		}
		if strings.TrimSpace(s.Content) == "" {
			problems = append(problems, fmt.Sprintf("- scene[%d]: \"content\" is empty", i))
		}
		if len(s.Items) == 0 {
			problems = append(problems, fmt.Sprintf("- scene[%d]: \"items\" is empty", i))
		}
		if s.StartTime < 0 {
			problems = append(problems, fmt.Sprintf("- scene[%d]: \"start_time\" is negative", i))
		}
		if s.EndTime <= s.StartTime {
			problems = append(problems, fmt.Sprintf("- scene[%d]: \"end_time\" (%g) must be greater than \"start_time\" (%g)", i, s.EndTime, s.StartTime))
		}
		if m.Duration > 0 && s.EndTime > m.Duration {
			problems = append(problems, fmt.Sprintf("- scene[%d]: \"end_time\" (%g) exceeds video duration (%g)", i, s.EndTime, m.Duration))
		}
	}

	if m.Duration > 0 && len(m.Scenes) > 0 {
		lastEnd := m.Scenes[len(m.Scenes)-1].EndTime
		if lastEnd < m.Duration*0.5 {
			problems = append(problems, fmt.Sprintf(
				"- the last scene ends at %g but duration is %g seconds — timestamps are likely not in seconds. "+
					"All timestamps (start_time, end_time) MUST be in seconds as plain numbers, NOT MM:SS or M.SS",
				lastEnd, m.Duration))
		}
	}

	return problems
}

// dumpAPIError extracts a genai.APIError from the error chain and logs the
// full details as JSON for debugging.
func (c *Client) dumpAPIError(context string, err error) {
	var apiErr genai.APIError
	if errors.As(err, &apiErr) {
		dump, _ := json.MarshalIndent(apiErr, "", "  ")
		c.logger.Printf("%s: Gemini API error (full dump):\n%s", context, dump)
	} else {
		c.logger.Printf("%s: non-API error: %v", context, err)
	}
}

// BuildDenseContent constructs a context-rich string for a single scene by
// combining video-level metadata with scene-level detail. The resulting string
// is intended to be embedded and stored in the content column so that vector
// search can distinguish scenes across different videos.
func BuildDenseContent(m *GeminiMetadata, s *SceneMetadata) string {
	return fmt.Sprintf(
		"Video: %s. Category: %s. Keywords: %s. Scene: %s. Items: %s.",
		m.VideoTitle,
		m.VideoCategory,
		strings.Join(m.GlobalKeywords, ", "),
		s.Content,
		strings.Join(s.Items, ", "),
	)
}

// GenerateEmbedding creates a 768-dimension vector for the given text.
func (c *Client) GenerateEmbedding(ctx context.Context, text string) (string, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  c.apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create genai client: %w", err)
	}

	content :=[]*genai.Content{
		{Parts:[]*genai.Part{{Text: text}}},
	}

	dims := int32(768)
	resp, err := client.Models.EmbedContent(ctx, c.embeddingModel, content, &genai.EmbedContentConfig{
		OutputDimensionality: &dims,
	})
	if err != nil {
		return "", fmt.Errorf("embed content failed: %w", err)
	}

	if len(resp.Embeddings) == 0 {
		return "", fmt.Errorf("no embeddings returned")
	}

	b, err := json.Marshal(resp.Embeddings[0].Values)
	if err != nil {
		return "", fmt.Errorf("failed to marshal embedding: %w", err)
	}

	return string(b), nil
}

// CHANGE: Moved retry logic from ai.Embedder to gemini.Client
// GenerateEmbeddingWithRetry wraps GenerateEmbedding with exponential backoff.
func (c *Client) GenerateEmbeddingWithRetry(ctx context.Context, text string, maxAttempts int) (string, error) {
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		result, err := c.GenerateEmbedding(ctx, text)
		if err == nil {
			return result, nil
		}
		lastErr = err
		c.logger.Printf("embedding attempt %d/%d failed: %v", attempt, maxAttempts, err)

		if attempt < maxAttempts {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(backoff):
			}
		}
	}
	return "", fmt.Errorf("all %d embedding attempts failed: %w", maxAttempts, lastErr)
}