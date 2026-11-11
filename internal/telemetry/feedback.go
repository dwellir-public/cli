package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// FeedbackEvent is the PostHog event that carries `dwellir feedback`.
// Destinations that route feedback to the team filter on this name.
const FeedbackEvent = "cli_feedback"

// ErrFeedbackUnavailable means this run has no channel for feedback: the build
// has no PostHog key, or DWELLIR_API_URL points away from production.
var ErrFeedbackUnavailable = errors.New("feedback channel unavailable")

var feedbackHTTPClient = &http.Client{Timeout: 15 * time.Second}

// SendFeedback captures one feedback event under the identity from Init and
// waits for PostHog to accept it. Usage events are batched and best effort.
// Feedback is sent at once, because the user typed it and must know whether
// it arrived. id becomes the event UUID, so the team can find the event.
func SendFeedback(ctx context.Context, id string, props map[string]interface{}) error {
	apiKey := posthogAPIKey
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("DWELLIR_POSTHOG_KEY"))
	}
	if !Enabled() || apiKey == "" {
		return ErrFeedbackUnavailable
	}

	properties := baseProperties()
	for k, v := range props {
		properties.Set(k, v)
	}
	if g := groups(); g != nil {
		properties.Set("$groups", g)
	}
	body, err := json.Marshal(map[string]interface{}{
		"api_key":     apiKey,
		"event":       FeedbackEvent,
		"distinct_id": distinctID(),
		"uuid":        id,
		"timestamp":   time.Now().UTC().Format(time.RFC3339Nano),
		"properties":  properties,
	})
	if err != nil {
		return err
	}

	url := strings.TrimRight(resolveEndpoint(), "/") + "/i/v0/e/"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := feedbackHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("feedback service answered HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	return nil
}
