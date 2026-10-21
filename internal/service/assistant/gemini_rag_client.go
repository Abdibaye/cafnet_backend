package assistant

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/Abdibaye/cafnet_backend/internal/config"
	"github.com/Abdibaye/cafnet_backend/internal/domain"
)

// GeminiRAGClient provides embedding generation and grounded question answering via Google Gemini API.
type GeminiRAGClient interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	BatchEmbed(ctx context.Context, texts []string) ([][]float32, error)
	GenerateGroundedAnswer(
		ctx context.Context,
		history []domain.AssistantMessage,
		userQuery string,
		retrievedDocs []domain.RetrievedDocument,
	) (answer string, followUps []string, err error)
}

type geminiRAGClient struct {
	cfg        config.AIConfig
	httpClient *http.Client
}

// NewGeminiRAGClient constructs a GeminiRAGClient.
func NewGeminiRAGClient(cfg config.AIConfig) GeminiRAGClient {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &geminiRAGClient{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// Embed computes a 768-dimensional embedding vector for the input string.
func (c *geminiRAGClient) Embed(ctx context.Context, text string) ([]float32, error) {
	if !c.cfg.HasAPIKey() {
		return deterministicEmbedding(text, 768), nil
	}

	endpoint := fmt.Sprintf(
		"%s/v1beta/models/%s:embedContent?key=%s",
		c.cfg.BaseURL,
		c.cfg.EmbeddingModel,
		c.cfg.GeminiAPIKey,
	)

	reqBody := map[string]interface{}{
		"model": fmt.Sprintf("models/%s", c.cfg.EmbeddingModel),
		"content": map[string]interface{}{
			"parts": []map[string]interface{}{
				{"text": text},
			},
		},
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal embed request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create embed request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		slog.Warn("gemini embed request failed, using deterministic embedding", "error", err)
		return deterministicEmbedding(text, 768), nil
	}
	defer httpResp.Body.Close()

	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read embed response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		slog.Warn("gemini embed returned non-200, using deterministic embedding", "status", httpResp.StatusCode, "body", string(bodyBytes))
		return deterministicEmbedding(text, 768), nil
	}

	var resp struct {
		Embedding struct {
			Values []float32 `json:"values"`
		} `json:"embedding"`
	}
	if err := json.Unmarshal(bodyBytes, &resp); err != nil {
		return nil, fmt.Errorf("failed to decode embed response: %w", err)
	}

	if len(resp.Embedding.Values) == 0 {
		return deterministicEmbedding(text, 768), nil
	}

	return resp.Embedding.Values, nil
}

// BatchEmbed vectorizes multiple text documents in a single or chunked request.
func (c *geminiRAGClient) BatchEmbed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	if !c.cfg.HasAPIKey() {
		results := make([][]float32, len(texts))
		for i, t := range texts {
			results[i] = deterministicEmbedding(t, 768)
		}
		return results, nil
	}

	// Process in batches of 20 to avoid exceeding payload limits
	const batchSize = 20
	var allEmbeddings [][]float32

	for i := 0; i < len(texts); i += batchSize {
		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}
		chunk := texts[i:end]

		chunkEmbeddings, err := c.embedBatchChunk(ctx, chunk)
		if err != nil {
			slog.Warn("batch embed chunk failed, falling back to individual/deterministic embedding", "error", err)
			for _, t := range chunk {
				emb, _ := c.Embed(ctx, t)
				allEmbeddings = append(allEmbeddings, emb)
			}
		} else {
			allEmbeddings = append(allEmbeddings, chunkEmbeddings...)
		}
	}

	return allEmbeddings, nil
}

func (c *geminiRAGClient) embedBatchChunk(ctx context.Context, texts []string) ([][]float32, error) {
	endpoint := fmt.Sprintf(
		"%s/v1beta/models/%s:batchEmbedContents?key=%s",
		c.cfg.BaseURL,
		c.cfg.EmbeddingModel,
		c.cfg.GeminiAPIKey,
	)

	var requests []map[string]interface{}
	for _, text := range texts {
		requests = append(requests, map[string]interface{}{
			"model": fmt.Sprintf("models/%s", c.cfg.EmbeddingModel),
			"content": map[string]interface{}{
				"parts": []map[string]interface{}{
					{"text": text},
				},
			},
		})
	}

	reqBody := map[string]interface{}{
		"requests": requests,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("status %d: %s", httpResp.StatusCode, string(b))
	}

	var resp struct {
		Embeddings []struct {
			Values []float32 `json:"values"`
		} `json:"embeddings"`
	}
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		return nil, err
	}

	var result [][]float32
	for _, emb := range resp.Embeddings {
		result = append(result, emb.Values)
	}
	return result, nil
}

const ragSystemPrompt = `You are Cafnet Food Assistant, a friendly and accurate dining guide for Addis Ababa, Ethiopia.
Your mission is to answer users' dining questions (e.g. where to eat a dish, within budget, opening hours, reservation policies, and locations).

STRICT GROUNDING RULES:
1. Answer ONLY using the facts present in the provided Cafnet restaurant documents below.
2. If the user asks for a dish, price, location, or restaurant that is NOT contained in the retrieved documents, clearly and politely acknowledge that Cafnet's verified listings do not currently have that exact match, and suggest the closest available alternative from the documents.
3. NEVER invent or hallucinate restaurants, menu items, prices, ratings, or policies.
4. Always state prices in Ethiopian Birr (ETB).
5. At the end of your answer, include 2-3 short, relevant follow-up suggestions formatted strictly as:
FOLLOW_UPS:
- Suggestion 1
- Suggestion 2
- Suggestion 3`

// GenerateGroundedAnswer invokes Gemini generateContent with retrieved documents as grounded context.
func (c *geminiRAGClient) GenerateGroundedAnswer(
	ctx context.Context,
	history []domain.AssistantMessage,
	userQuery string,
	retrievedDocs []domain.RetrievedDocument,
) (string, []string, error) {
	if !c.cfg.HasAPIKey() {
		return c.synthesizeLocalFallback(userQuery, retrievedDocs)
	}

	// 1. Build document context block
	var docContext strings.Builder
	docContext.WriteString("VERIFIED CAFNET RESTAURANT DOCUMENTS:\n\n")

	if len(retrievedDocs) == 0 {
		docContext.WriteString("(No matching restaurant documents found in Cafnet database for this specific query.)\n\n")
	} else {
		for i, doc := range retrievedDocs {
			docContext.WriteString(fmt.Sprintf("--- DOCUMENT [%d] (%s) ---\n", i+1, doc.Document.Title))
			docContext.WriteString(doc.Document.Content)
			docContext.WriteString("\n\n")
		}
	}

	// 2. Build conversational contents
	var contents []map[string]interface{}

	// Include up to last 4 conversation turns for context
	startIdx := 0
	if len(history) > 4 {
		startIdx = len(history) - 4
	}
	for _, m := range history[startIdx:] {
		role := "user"
		if m.Role == "assistant" {
			role = "model"
		}
		contents = append(contents, map[string]interface{}{
			"role": role,
			"parts": []map[string]interface{}{
				{"text": m.Content},
			},
		})
	}

	// Current turn with user prompt and injected document context
	userTurnText := fmt.Sprintf("%s\nUSER QUESTION: %s", docContext.String(), userQuery)
	contents = append(contents, map[string]interface{}{
		"role": "user",
		"parts": []map[string]interface{}{
			{"text": userTurnText},
		},
	})

	endpoint := fmt.Sprintf(
		"%s/v1beta/models/%s:generateContent?key=%s",
		c.cfg.BaseURL,
		c.cfg.Model,
		c.cfg.GeminiAPIKey,
	)

	reqPayload := map[string]interface{}{
		"system_instruction": map[string]interface{}{
			"parts": []map[string]interface{}{
				{"text": ragSystemPrompt},
			},
		},
		"contents": contents,
		"generationConfig": map[string]interface{}{
			"temperature":     0.2,
			"maxOutputTokens": 800,
		},
	}

	bodyBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return "", nil, fmt.Errorf("failed to marshal generateContent request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", nil, fmt.Errorf("failed to create generateContent request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		slog.Warn("gemini generateContent request failed, falling back to grounded local synthesis", "error", err)
		return c.synthesizeLocalFallback(userQuery, retrievedDocs)
	}
	defer httpResp.Body.Close()

	respBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read generateContent response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		slog.Warn("gemini generateContent non-200, falling back to local synthesis", "status", httpResp.StatusCode, "body", string(respBytes))
		return c.synthesizeLocalFallback(userQuery, retrievedDocs)
	}

	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.Unmarshal(respBytes, &geminiResp); err != nil {
		return "", nil, fmt.Errorf("failed to parse generateContent response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return c.synthesizeLocalFallback(userQuery, retrievedDocs)
	}

	rawAnswer := geminiResp.Candidates[0].Content.Parts[0].Text
	ans, followUps := parseAnswerAndFollowUps(rawAnswer)
	return ans, followUps, nil
}

func (c *geminiRAGClient) synthesizeLocalFallback(
	userQuery string,
	retrievedDocs []domain.RetrievedDocument,
) (string, []string, error) {
	if len(retrievedDocs) == 0 {
		return fmt.Sprintf("I searched Cafnet's verified listings for '%s', but could not find matching restaurants or dishes in that criteria. You can try searching with a wider budget or a different neighborhood in Addis Ababa.", userQuery),
			[]string{"Show popular places in Bole", "Show traditional Ethiopian kitfo", "Show budget options under 300 ETB"}, nil
	}

	var sb strings.Builder
	sb.WriteString("Here are verified options from Cafnet's restaurant directory:\n\n")

	for i, r := range retrievedDocs {
		if i >= 3 {
			break
		}
		doc := r.Document
		sb.WriteString(fmt.Sprintf("**%s**\n", doc.Title))
		lines := strings.Split(doc.Content, "\n")
		for j, l := range lines {
			if j > 4 {
				break
			}
			if strings.TrimSpace(l) != "" {
				sb.WriteString(fmt.Sprintf("• %s\n", l))
			}
		}
		sb.WriteString("\n")
	}

	followUps := []string{
		"Show opening hours",
		"Can I reserve a table?",
		"Show options under 300 ETB",
	}

	return strings.TrimSpace(sb.String()), followUps, nil
}

func parseAnswerAndFollowUps(raw string) (string, []string) {
	parts := strings.Split(raw, "FOLLOW_UPS:")
	answer := strings.TrimSpace(parts[0])

	var followUps []string
	if len(parts) > 1 {
		lines := strings.Split(parts[1], "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			trimmed = strings.TrimPrefix(trimmed, "- ")
			trimmed = strings.TrimPrefix(trimmed, "* ")
			trimmed = strings.TrimPrefix(trimmed, "• ")
			if trimmed != "" {
				followUps = append(followUps, trimmed)
			}
		}
	}

	if len(followUps) == 0 {
		followUps = []string{"Show opening hours", "Can I book a table?", "Show options in Bole"}
	}

	return answer, followUps
}

// deterministicEmbedding produces a stable, normalized 768-float pseudo-embedding from text hash
// for local testing and offline operation without an API key.
func deterministicEmbedding(text string, dim int) []float32 {
	vec := make([]float32, dim)
	h := sha256.New()
	h.Write([]byte(text))
	seed := h.Sum(nil)

	var norm float64
	for i := 0; i < dim; i++ {
		// Derive float deterministically
		idx := (i * 4) % len(seed)
		var val uint32
		if idx+4 <= len(seed) {
			val = binary.LittleEndian.Uint32(seed[idx : idx+4])
		} else {
			val = binary.LittleEndian.Uint32(seed[:4])
		}
		f := float32(float64(val%10000)/10000.0 - 0.5)
		vec[i] = f
		norm += float64(f * f)
	}

	// L2-normalize vector for cosine similarity
	sqrtNorm := float32(math.Sqrt(norm))
	if sqrtNorm > 0 {
		for i := range vec {
			vec[i] /= sqrtNorm
		}
	}
	return vec
}
