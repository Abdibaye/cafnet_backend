package assistant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Abdibaye/cafnet_backend/internal/config"
	"github.com/Abdibaye/cafnet_backend/internal/domain"
)

type geminiClient struct {
	cfg        config.AIConfig
	httpClient *http.Client
}

// NewGeminiClient creates an AIParser that queries Gemini or OpenAI APIs with structured output.
func NewGeminiClient(cfg config.AIConfig) AIParser {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	return &geminiClient{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

const systemPrompt = `You are Cafnet Food Assistant, a dining discovery guide for Addis Ababa, Ethiopia.
Your mission is to understand what dish or food the customer wants to eat, their budget in Ethiopian Birr (ETB), their preferred neighborhood or area in Addis Ababa (e.g. Bole, Kazanchis, Piazza, Sarbet, Megenagna, CMC, Gerji, Mexico, 22), and any rating preference.
Extract and output strictly valid JSON matching this schema:
{
  "dish_or_food": string, // Normalized food/dish name (e.g. "Tibs", "Kitfo", "Shiro", "Burger", "Pizza"). Empty string if no specific dish.
  "max_price_etb": number | null, // Maximum price in ETB (e.g. 300)
  "min_price_etb": number | null, // Minimum price in ETB
  "min_rating": number | null, // Minimum rating (e.g. 4.0 if asking for best/top)
  "neighborhood": string | null, // Area in Addis Ababa (e.g. "Bole", "Kazanchis"). Empty string if none.
  "intent": "food_search" | "clarification" | "general_qa",
  "reply_text": string, // Helpful 1-2 sentence response explaining what you found or asking clarification
  "follow_up_suggestions": string[] // 2-3 relevant follow-up query suggestions
}
Do NOT invent restaurant names or food prices. Only extract search parameters.`

func (c *geminiClient) Interpret(ctx context.Context, history []domain.AssistantMessage, userMessage string, userLocation string) (*InterpretedQuery, error) {
	if c.cfg.GeminiAPIKey != "" {
		return c.callGemini(ctx, history, userMessage, userLocation)
	} else if c.cfg.OpenAIAPIKey != "" {
		return c.callOpenAI(ctx, history, userMessage, userLocation)
	}
	return nil, fmt.Errorf("no AI API key configured")
}

func (c *geminiClient) callGemini(ctx context.Context, history []domain.AssistantMessage, userMessage string, userLocation string) (*InterpretedQuery, error) {
	model := c.cfg.Model
	if model == "" {
		model = "gemini-1.5-flash"
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, c.cfg.GeminiAPIKey)
	if c.cfg.BaseURL != "" {
		url = fmt.Sprintf("%s/models/%s:generateContent?key=%s", strings.TrimRight(c.cfg.BaseURL, "/"), model, c.cfg.GeminiAPIKey)
	}

	prompt := userMessage
	if userLocation != "" {
		prompt = fmt.Sprintf("%s (User location: %s)", userMessage, userLocation)
	}

	payload := map[string]interface{}{
		"system_instruction": map[string]interface{}{
			"parts": []map[string]string{{"text": systemPrompt}},
		},
		"contents": []map[string]interface{}{
			{
				"role": "user",
				"parts": []map[string]string{{"text": prompt}},
			},
		},
		"generationConfig": map[string]interface{}{
			"response_mime_type": "application/json",
			"temperature":        0.1,
		},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("gemini API returned status %d: %s", res.StatusCode, string(respBody))
	}

	var geminiResp geminiResponse
	if err := json.NewDecoder(res.Body).Decode(&geminiResp); err != nil {
		return nil, err
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty gemini response candidates")
	}

	jsonText := geminiResp.Candidates[0].Content.Parts[0].Text
	return parseAIJSONResponse(jsonText)
}

func (c *geminiClient) callOpenAI(ctx context.Context, history []domain.AssistantMessage, userMessage string, userLocation string) (*InterpretedQuery, error) {
	model := c.cfg.Model
	if model == "" {
		model = "gpt-4o-mini"
	}

	url := "https://api.openai.com/v1/chat/completions"
	if c.cfg.BaseURL != "" {
		url = fmt.Sprintf("%s/chat/completions", strings.TrimRight(c.cfg.BaseURL, "/"))
	}

	prompt := userMessage
	if userLocation != "" {
		prompt = fmt.Sprintf("%s (User location: %s)", userMessage, userLocation)
	}

	messages := []map[string]string{
		{"role": "system", "content": systemPrompt},
		{"role": "user", "content": prompt},
	}

	payload := map[string]interface{}{
		"model":       model,
		"messages":    messages,
		"temperature": 0.1,
		"response_format": map[string]string{
			"type": "json_object",
		},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.OpenAIAPIKey)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("openai API returned status %d: %s", res.StatusCode, string(respBody))
	}

	var openAIResp openAIResponse
	if err := json.NewDecoder(res.Body).Decode(&openAIResp); err != nil {
		return nil, err
	}

	if len(openAIResp.Choices) == 0 {
		return nil, fmt.Errorf("empty openai response choices")
	}

	jsonText := openAIResp.Choices[0].Message.Content
	return parseAIJSONResponse(jsonText)
}

func parseAIJSONResponse(jsonText string) (*InterpretedQuery, error) {
	trimmed := strings.TrimSpace(jsonText)
	// Strip markdown code fences if present (e.g. ```json ... ```)
	if strings.HasPrefix(trimmed, "```") {
		lines := strings.Split(trimmed, "\n")
		if len(lines) >= 2 {
			lines = lines[1:]
			if strings.HasPrefix(lines[len(lines)-1], "```") {
				lines = lines[:len(lines)-1]
			}
			trimmed = strings.Join(lines, "\n")
		}
	}

	var query InterpretedQuery
	if err := json.Unmarshal([]byte(trimmed), &query); err != nil {
		return nil, fmt.Errorf("failed to unmarshal AI structured output: %w (raw: %s)", err, trimmed)
	}

	query.FallbackUsed = false
	if query.Intent == "" {
		query.Intent = "food_search"
	}
	return &query, nil
}
