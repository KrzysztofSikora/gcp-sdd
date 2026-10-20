package analyzer

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

const contractResponse = `{"word_count": 5, "character_count": 20, "top_words": [
	{"word": "the", "count": 2}, {"word": "and", "count": 1},
	{"word": "cat", "count": 1}, {"word": "hat", "count": 1}]}`

func server(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func respond(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}
}

func TestAnalyzeSendsTextAndDecodesResult(t *testing.T) {
	var got struct {
		method, path, contentType string
		body                      map[string]any
	}
	srv := server(t, func(w http.ResponseWriter, r *http.Request) {
		got.method, got.path, got.contentType = r.Method, r.URL.Path, r.Header.Get("Content-Type")
		_ = json.NewDecoder(r.Body).Decode(&got.body)
		_, _ = io.WriteString(w, contractResponse)
	})

	result, err := New(srv.URL, time.Second, nil).Analyze(context.Background(), "The cat and the hat.")
	if err != nil {
		t.Fatal(err)
	}
	if got.method != http.MethodPost || got.path != "/analyze" || got.contentType != "application/json" {
		t.Errorf("request = %s %s (%s)", got.method, got.path, got.contentType)
	}
	if !reflect.DeepEqual(got.body, map[string]any{"text": "The cat and the hat."}) {
		t.Errorf("request body = %v", got.body)
	}
	want := Result{WordCount: 5, CharacterCount: 20, TopWords: []WordCount{
		{"the", 2}, {"and", 1}, {"cat", 1}, {"hat", 1},
	}}
	if !reflect.DeepEqual(result, want) {
		t.Errorf("result = %+v, want %+v", result, want)
	}
}

func TestAnalyzeEmptyTopWords(t *testing.T) {
	srv := server(t, respond(200, `{"word_count":0,"character_count":7,"top_words":[]}`))
	result, err := New(srv.URL, time.Second, nil).Analyze(context.Background(), "!!! ???")
	if err != nil {
		t.Fatal(err)
	}
	if result.TopWords == nil || len(result.TopWords) != 0 {
		t.Errorf("top words = %#v, want empty non-nil", result.TopWords)
	}
}

func TestAnalyzeBadResponses(t *testing.T) {
	cases := map[string]http.HandlerFunc{
		"status 500":      respond(500, `{"detail":"boom"}`),
		"status 422":      respond(422, `{"detail":[]}`),
		"not json":        respond(200, `<html>`),
		"missing field":   respond(200, `{"word_count":1,"character_count":1}`),
		"null top words":  respond(200, `{"word_count":1,"character_count":1,"top_words":null}`),
		"negative count":  respond(200, `{"word_count":-1,"character_count":1,"top_words":[]}`),
		"too many words":  respond(200, `{"word_count":6,"character_count":11,"top_words":[{"word":"a","count":1},{"word":"b","count":1},{"word":"c","count":1},{"word":"d","count":1},{"word":"e","count":1},{"word":"f","count":1}]}`),
		"empty word":      respond(200, `{"word_count":1,"character_count":1,"top_words":[{"word":"","count":1}]}`),
		"zero word count": respond(200, `{"word_count":1,"character_count":1,"top_words":[{"word":"a","count":0}]}`),
	}
	for name, handler := range cases {
		t.Run(name, func(t *testing.T) {
			srv := server(t, handler)
			_, err := New(srv.URL, time.Second, nil).Analyze(context.Background(), "a")
			if !errors.Is(err, ErrBadResponse) {
				t.Fatalf("err = %v, want ErrBadResponse", err)
			}
		})
	}
}

func TestAnalyzeTimeout(t *testing.T) {
	release := make(chan struct{})
	srv := server(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	})
	defer close(release)

	start := time.Now()
	_, err := New(srv.URL, 50*time.Millisecond, nil).Analyze(context.Background(), "a")
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("err = %v, want ErrTimeout", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("took %v, want about the timeout", elapsed)
	}
}

func TestAnalyzeUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close() // nothing listens there any more

	_, err := New(url, time.Second, nil).Analyze(context.Background(), "a")
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v, want ErrUnavailable", err)
	}
}

func TestAnalyzeDoesNotRetry(t *testing.T) {
	calls := 0
	srv := server(t, func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(503)
	})
	_, _ = New(srv.URL, time.Second, nil).Analyze(context.Background(), "a")
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}
