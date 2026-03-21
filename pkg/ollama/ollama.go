package ollama

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"time"
)

const (
	DefaultURL   = "http://localhost:11434"
	DefaultModel = "qwen3-embedding:0.6b"
)

type Client struct {
	BaseURL string
	Model   string
	HTTP    *http.Client
}

func NewClient(baseURL, model string) *Client {
	if baseURL == "" {
		baseURL = DefaultURL
	}
	if model == "" {
		model = DefaultModel
	}
	return &Client{
		BaseURL: baseURL,
		Model:   model,
		HTTP:    &http.Client{Timeout: 60 * time.Second},
	}
}

type embedRequest struct {
	Model string `json:"model"`
	Input any    `json:"input"`
}

type embedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
	Error      string      `json:"error,omitempty"`
}

var errContextLength = fmt.Errorf("input length exceeds context length")

func (c *Client) embedRaw(input any) (*embedResponse, error) {
	body, err := json.Marshal(embedRequest{Model: c.Model, Input: input})
	if err != nil {
		return nil, fmt.Errorf("marshal embed request: %w", err)
	}

	resp, err := c.HTTP.Post(c.BaseURL+"/api/embed", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ollama request failed (is Ollama running?): %w", err)
	}
	defer resp.Body.Close()

	var result embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("ollama returned status %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("decode ollama response: %w", err)
	}

	if result.Error != "" {
		if strings.Contains(result.Error, "context length") {
			return nil, errContextLength
		}
		return nil, fmt.Errorf("ollama error: %s", result.Error)
	}

	return &result, nil
}

// EmbedOne embeds a single text, auto-reducing if it exceeds context length.
func (c *Client) EmbedOne(text string) ([]float32, error) {
	originalLen := len(text)
	for {
		result, err := c.embedRaw(text)
		if err == errContextLength && len(text) > 8000 {
			newLen := len(text) * 2 / 3
			slog.Warn("embedding input too large, reducing",
				"original_chars", originalLen,
				"from_chars", len(text),
				"to_chars", newLen)
			text = text[:newLen]
			continue
		}
		if err != nil {
			return nil, err
		}
		if len(result.Embeddings) == 0 {
			return nil, fmt.Errorf("ollama returned no embeddings")
		}
		return result.Embeddings[0], nil
	}
}

// Float32ToBytes converts float32 slice to raw bytes for storage.
func Float32ToBytes(f []float32) []byte {
	buf := make([]byte, len(f)*4)
	for i, v := range f {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	return buf
}

// BytesToFloat32 converts raw bytes back to float32 slice.
func BytesToFloat32(b []byte) []float32 {
	f := make([]float32, len(b)/4)
	for i := range f {
		f[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return f
}

// CosineSimilarity computes cosine similarity between two vectors.
func CosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float32
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / float32(math.Sqrt(float64(normA))*math.Sqrt(float64(normB)))
}
