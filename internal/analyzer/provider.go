package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Provider completes a chat exchange and returns the model's JSON object.
type Provider interface {
	Name() string
	CompleteJSON(ctx context.Context, messages []Message) (map[string]any, error)
}

// ProviderError is a transport or model failure from one provider. Retryable
// errors let a relay chain fall through to the next provider.
type ProviderError struct {
	Message    string
	Retryable  bool
	StatusCode int
}

func (e *ProviderError) Error() string { return e.Message }

// extractJSONText strips a markdown code fence if the model wrapped its JSON.
func extractJSONText(raw string) string {
	s := strings.TrimSpace(raw)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(strings.TrimSpace(s), "```")
	return strings.TrimSpace(s)
}

func parseJSONObject(content, providerName string) (map[string]any, error) {
	if content == "" {
		return nil, &ProviderError{Message: fmt.Sprintf("%s returned an empty message.", providerName)}
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(extractJSONText(content)), &parsed); err != nil {
		return nil, &ProviderError{Message: fmt.Sprintf("%s returned invalid JSON.", providerName)}
	}
	return parsed, nil
}

// Relay tries each provider in order, falling through on retryable failures.
type Relay struct {
	providers []Provider
}

func NewRelay(providers ...Provider) *Relay { return &Relay{providers: providers} }

func (r *Relay) Name() string { return "Relay" }

func (r *Relay) CompleteJSON(ctx context.Context, messages []Message) (map[string]any, error) {
	var errs []string
	for _, p := range r.providers {
		result, err := p.CompleteJSON(ctx, messages)
		if err == nil {
			return result, nil
		}
		errs = append(errs, fmt.Sprintf("%s: %v", p.Name(), err))
		var pe *ProviderError
		if !asProviderError(err, &pe) || !pe.Retryable {
			return nil, err
		}
		slog.Warn("ai provider failed, trying next", "provider", p.Name(), "err", err)
	}
	return nil, &ProviderError{Message: "All AI relay providers failed: " + strings.Join(errs, " | ")}
}

func asProviderError(err error, target **ProviderError) bool {
	pe, ok := err.(*ProviderError)
	if ok {
		*target = pe
	}
	return ok
}
