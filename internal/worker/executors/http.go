// executors/http.go: "http" type task ka executor.
// Payload JSON format: {"method": "POST", "url": "...", "body": {...}}
package executors

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// HTTPPayload: task.Payload (JSON bytes) isi struct me unmarshal hota hai.
type HTTPPayload struct {
	Method string          `json:"method"`
	URL    string          `json:"url"`
	Body   json.RawMessage `json:"body"`
}

// RunHTTP: payload parse karke HTTP request bhejta hai.
// idempotencyKey header me jaati hai, taaki target service retry pe
// duplicate request ko pehchaan sake.

func RunHTTP(payload []byte, idempotencyKey string) error {

	var p HTTPPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("invalid http payload: %w", err)
	}

	if p.URL == "" {
		return fmt.Errorf("http payload missing url")
	}

	method := p.Method
	if method == "" {
		method = http.MethodGet
	}

	var bodyReader io.Reader
	if len(p.Body) > 0 {
		bodyReader = bytes.NewReader(p.Body)
	}

	req, err := http.NewRequest(method, p.URL, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("http request returned status %d", resp.StatusCode)
	}

	return nil
}
