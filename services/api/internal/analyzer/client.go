// Package analyzer is the HTTP client of the private analyzer service
// (specs/design.md, private service contract).
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
	"time"
)

// Errors returned by Client.Analyze, wrapped with details. The HTTP layer maps them to
// 504 analyzer_timeout, 502 analyzer_unavailable and 502 analyzer_error (FR-04).
var (
	ErrTimeout     = errors.New("analyzer timed out")
	ErrUnavailable = errors.New("analyzer unavailable")
	ErrBadResponse = errors.New("analyzer returned an invalid response")
)

const (
	maxTopWords      = 5
	maxResponseBytes = 1 << 20
)

// WordCount is one entry of Result.TopWords.
type WordCount struct {
	Word  string `json:"word"`
	Count int    `json:"count"`
}

// Result is a validated analysis result.
type Result struct {
	WordCount      int         `json:"word_count"`
	CharacterCount int         `json:"character_count"`
	TopWords       []WordCount `json:"top_words"`
}

// Client calls POST <base URL>/analyze. It is safe for concurrent use.
type Client struct {
	endpoint string
	timeout  time.Duration
	http     *http.Client
}

// New returns a client for the analyzer at baseURL. A nil transport uses
// http.DefaultTransport.
func New(baseURL string, timeout time.Duration, transport http.RoundTripper) *Client {
	if transport == nil {
		transport = http.DefaultTransport
	}
	return &Client{
		endpoint: baseURL + "/analyze",
		timeout:  timeout,
		http:     &http.Client{Transport: transport},
	}
}

// Analyze sends text to the analyzer once (no retries) and validates the answer.
func (c *Client) Analyze(ctx context.Context, text string) (Result, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	body, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return Result{}, fmt.Errorf("%w: encode request: %v", ErrBadResponse, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return Result{}, fmt.Errorf("%w: build request: %v", ErrUnavailable, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return Result{}, classify(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("%w: status %d", ErrBadResponse, resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return Result{}, classify(err)
	}
	return decode(raw)
}

func classify(err error) error {
	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()) {
		return fmt.Errorf("%w: %v", ErrTimeout, err)
	}
	return fmt.Errorf("%w: %v", ErrUnavailable, err)
}

// wire mirrors Result with pointers, so that missing fields are told apart from zeros.
type wire struct {
	WordCount      *int `json:"word_count"`
	CharacterCount *int `json:"character_count"`
	TopWords       *[]struct {
		Word  *string `json:"word"`
		Count *int    `json:"count"`
	} `json:"top_words"`
}

func decode(raw []byte) (Result, error) {
	var w wire
	if err := json.Unmarshal(raw, &w); err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrBadResponse, err)
	}
	bad := func(reason string) (Result, error) {
		return Result{}, fmt.Errorf("%w: %s", ErrBadResponse, reason)
	}
	switch {
	case w.WordCount == nil || w.CharacterCount == nil || w.TopWords == nil:
		return bad("missing field")
	case *w.WordCount < 0 || *w.CharacterCount < 0:
		return bad("negative count")
	case len(*w.TopWords) > maxTopWords:
		return bad("more than 5 top words")
	}
	result := Result{
		WordCount:      *w.WordCount,
		CharacterCount: *w.CharacterCount,
		TopWords:       make([]WordCount, 0, len(*w.TopWords)),
	}
	for _, tw := range *w.TopWords {
		if tw.Word == nil || *tw.Word == "" || tw.Count == nil || *tw.Count < 1 {
			return bad("invalid top word")
		}
		result.TopWords = append(result.TopWords, WordCount{Word: *tw.Word, Count: *tw.Count})
	}
	return result, nil
}
