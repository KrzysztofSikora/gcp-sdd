// Package httpapi is the public HTTP layer of the Text Analysis API (specs/design.md).
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/krzysztofsikora/gcp-sdd/services/api/internal/analyzer"
)

// Analyzer performs the text analysis; *analyzer.Client implements it.
type Analyzer interface {
	Analyze(ctx context.Context, text string) (analyzer.Result, error)
}

type wordCount struct {
	Word  string `json:"word"`
	Count int    `json:"count"`
}

// analyzeResponse is the public response; it is ours, not a copy of the analyzer's bytes (D-05).
type analyzeResponse struct {
	WordCount      int         `json:"word_count"`
	CharacterCount int         `json:"character_count"`
	TopWords       []wordCount `json:"top_words"`
}

// NewHandler returns the API's routes wrapped in request logging and panic recovery.
func NewHandler(a Analyzer, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/analyze", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeError(w, errMethodNotAllowed)
			return
		}
		handleAnalyze(w, r, a, logger)
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			writeError(w, errMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, errNotFound)
	})
	return logRequests(logger, recoverPanics(logger, mux))
}

func handleAnalyze(w http.ResponseWriter, r *http.Request, a Analyzer, logger *slog.Logger) {
	text, apiErr := decodeText(w, r)
	if apiErr != nil {
		writeError(w, apiErr)
		return
	}

	result, err := a.Analyze(r.Context(), text)
	if err != nil {
		// The error names the failure class and the analyzer URL, never the text (NFR-02).
		logger.Warn("analyzer call failed", "error", err.Error())
		switch {
		case errors.Is(err, analyzer.ErrTimeout):
			writeError(w, errAnalyzerTimeout)
		case errors.Is(err, analyzer.ErrUnavailable):
			writeError(w, errAnalyzerUnavailable)
		default:
			writeError(w, errAnalyzerError)
		}
		return
	}

	resp := analyzeResponse{
		WordCount:      result.WordCount,
		CharacterCount: result.CharacterCount,
		TopWords:       make([]wordCount, 0, len(result.TopWords)),
	}
	for _, tw := range result.TopWords {
		resp.TopWords = append(resp.TopWords, wordCount{Word: tw.Word, Count: tw.Count})
	}
	writeJSON(w, http.StatusOK, resp)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(status int) {
	s.status = status
	s.ResponseWriter.WriteHeader(status)
}

// logRequests writes one log line per request: method, path, status, duration (NFR-02).
func logRequests(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

func recoverPanics(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if p := recover(); p != nil {
				if p == http.ErrAbortHandler {
					panic(p)
				}
				logger.Error("panic", "panic", p)
				writeError(w, errInternal)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
