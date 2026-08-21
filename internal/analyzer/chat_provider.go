package analyzer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

var retryableStatusCodes = map[int]bool{402: true, 408: true, 409: true, 425: true, 429: true}

// ChatProviderConfig describes one OpenAI-compatible chat-completions provider.
type ChatProviderConfig struct {
	Name         string
	ServerURL    string
	APIKey       string
	Model        string
	Timeout      time.Duration
	ExtraHeaders map[string]string
}

type chatProvider struct {
	cfg    ChatProviderConfig
	client *http.Client
}

// NewChatProvider builds a provider speaking the /chat/completions protocol.
func NewChatProvider(cfg ChatProviderConfig) (Provider, error) {
	if cfg.APIKey == "" {
		return nil, &ProviderError{Message: fmt.Sprintf("%s API key is not configured.", cfg.Name)}
	}
	if cfg.Model == "" {
		return nil, &ProviderError{Message: fmt.Sprintf("No model configured for %s.", cfg.Name)}
	}
	cfg.ServerURL = strings.TrimRight(cfg.ServerURL, "/")
	// Timeout guards each phase (dial, TLS, response headers) rather than the
	// whole exchange: LLM providers stream slow bodies well past the header
	// phase, and a total-request timeout would cut them off mid-read. A hard
	// cap on the full exchange is applied per request in CompleteJSON.
	client := &http.Client{Transport: &http.Transport{
		DialContext:           (&net.Dialer{Timeout: cfg.Timeout}).DialContext,
		TLSHandshakeTimeout:   cfg.Timeout,
		ResponseHeaderTimeout: cfg.Timeout,
	}}
	return &chatProvider{cfg: cfg, client: client}, nil
}

func (p *chatProvider) Name() string { return p.cfg.Name }

func (p *chatProvider) CompleteJSON(ctx context.Context, messages []Message) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(ctx, 6*p.cfg.Timeout)
	defer cancel()
	body, err := json.Marshal(map[string]any{
		"model":           p.cfg.Model,
		"messages":        messages,
		"response_format": map[string]string{"type": "json_object"},
		"temperature":     0.2,
	})
	if err != nil {
		return nil, &ProviderError{Message: err.Error()}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.ServerURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, &ProviderError{Message: err.Error()}
	}
	req.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range p.cfg.ExtraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		msg := fmt.Sprintf("%s request failed: %v", p.cfg.Name, err)
		if errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "Client.Timeout") {
			msg = fmt.Sprintf("%s request timed out.", p.cfg.Name)
		}
		return nil, &ProviderError{Message: msg, Retryable: true}
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, &ProviderError{Message: fmt.Sprintf("%s response read failed: %v", p.cfg.Name, err), Retryable: true}
	}

	if resp.StatusCode >= 400 {
		return nil, &ProviderError{
			Message:    fmt.Sprintf("%s returned HTTP %d: %s", p.cfg.Name, resp.StatusCode, errorDetail(raw)),
			Retryable:  retryableStatusCodes[resp.StatusCode] || resp.StatusCode >= 500,
			StatusCode: resp.StatusCode,
		}
	}

	var payload struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, &ProviderError{Message: fmt.Sprintf("%s returned invalid JSON.", p.cfg.Name)}
	}
	content := ""
	if len(payload.Choices) > 0 {
		content = payload.Choices[0].Message.Content
	}
	return parseJSONObject(content, p.cfg.Name)
}

func errorDetail(raw []byte) string {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		s := string(raw)
		if len(s) > 500 {
			s = s[:500]
		}
		return s
	}
	if errObj, ok := payload["error"].(map[string]any); ok {
		if msg, ok := errObj["message"].(string); ok && msg != "" {
			return msg
		}
	}
	if errStr, ok := payload["error"].(string); ok {
		return errStr
	}
	if msg, ok := payload["message"].(string); ok && msg != "" {
		return msg
	}
	s := fmt.Sprintf("%v", payload)
	if len(s) > 500 {
		s = s[:500]
	}
	return s
}
