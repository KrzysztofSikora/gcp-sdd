package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/krzysztofsikora/gcp-sdd/services/api/internal/analyzer"
)

// fakeAnalyzer records calls and returns a fixed answer.
type fakeAnalyzer struct {
	calls  []string
	result analyzer.Result
	err    error
	panic  bool
}

func (f *fakeAnalyzer) Analyze(_ context.Context, text string) (analyzer.Result, error) {
	f.calls = append(f.calls, text)
	if f.panic {
		panic("boom")
	}
	return f.result, f.err
}

var contractResult = analyzer.Result{WordCount: 5, CharacterCount: 20, TopWords: []analyzer.WordCount{
	{Word: "the", Count: 2}, {Word: "and", Count: 1}, {Word: "cat", Count: 1}, {Word: "hat", Count: 1},
}}

func serve(h http.Handler, method, path, contentType, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func quietLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("error body %q: %v", rec.Body.String(), err)
	}
	if body.Error.Message == "" {
		t.Errorf("error message is empty")
	}
	return body.Error.Code
}

// FR-01 AC1, AC2, AC3
func TestAnalyzeSuccess(t *testing.T) {
	fake := &fakeAnalyzer{result: contractResult}
	rec := serve(NewHandler(fake, quietLogger()), "POST", "/analyze", "application/json",
		`{"text": "The cat and the hat."}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	want := `{"word_count":5,"character_count":20,"top_words":[{"word":"the","count":2},{"word":"and","count":1},{"word":"cat","count":1},{"word":"hat","count":1}]}` + "\n"
	if rec.Body.String() != want {
		t.Errorf("body = %s, want %s", rec.Body, want)
	}
	if len(fake.calls) != 1 || fake.calls[0] != "The cat and the hat." {
		t.Errorf("analyzer calls = %q", fake.calls)
	}
}

func TestAnalyzeEmptyTopWordsIsArray(t *testing.T) {
	fake := &fakeAnalyzer{result: analyzer.Result{WordCount: 0, CharacterCount: 7}}
	rec := serve(NewHandler(fake, quietLogger()), "POST", "/analyze", "application/json", `{"text":"!!! ???"}`)
	if !strings.Contains(rec.Body.String(), `"top_words":[]`) {
		t.Errorf("body = %s, want top_words []", rec.Body)
	}
}

func TestAnalyzeCharsetParameterAllowed(t *testing.T) {
	fake := &fakeAnalyzer{result: contractResult}
	rec := serve(NewHandler(fake, quietLogger()), "POST", "/analyze", "application/json; charset=utf-8", `{"text":"a"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestAnalyzeMaxLengthAccepted(t *testing.T) {
	fake := &fakeAnalyzer{result: contractResult}
	text := strings.Repeat("ż", maxTextLength) // multi-byte: counts code points, not bytes
	rec := serve(NewHandler(fake, quietLogger()), "POST", "/analyze", "application/json", fmt.Sprintf(`{"text":%q}`, text))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
}

// FR-03 AC1–AC8: every invalid request gets a JSON error and never reaches the analyzer.
func TestAnalyzeValidation(t *testing.T) {
	cases := []struct {
		name, method, contentType, body string
		status                          int
		code                            string
	}{
		{"AC1 GET", "GET", "", "", 405, "method_not_allowed"},
		{"AC1 PUT", "PUT", "application/json", `{"text":"a"}`, 405, "method_not_allowed"},
		{"AC2 no content type", "POST", "", `{"text":"a"}`, 415, "unsupported_media_type"},
		{"AC2 text/plain", "POST", "text/plain", `{"text":"a"}`, 415, "unsupported_media_type"},
		{"AC3 body too large", "POST", "application/json", `{"text":"` + strings.Repeat("a", maxBodyBytes) + `"}`, 413, "payload_too_large"},
		{"AC4 malformed", "POST", "application/json", `{"text":`, 400, "invalid_json"},
		{"AC4 empty body", "POST", "application/json", ``, 400, "invalid_json"},
		{"AC4 array", "POST", "application/json", `["a"]`, 400, "invalid_json"},
		{"AC4 null", "POST", "application/json", `null`, 400, "invalid_json"},
		{"AC4 string", "POST", "application/json", `"a"`, 400, "invalid_json"},
		{"AC4 trailing data", "POST", "application/json", `{"text":"a"} {}`, 400, "invalid_json"},
		{"AC5 unknown field", "POST", "application/json", `{"text":"a","lang":"en"}`, 400, "invalid_json"},
		{"AC6 missing text", "POST", "application/json", `{}`, 400, "invalid_text"},
		{"AC6 null text", "POST", "application/json", `{"text":null}`, 400, "invalid_text"},
		{"AC6 number text", "POST", "application/json", `{"text":42}`, 400, "invalid_text"},
		{"AC6 object text", "POST", "application/json", `{"text":{"a":1}}`, 400, "invalid_text"},
		{"AC7 empty text", "POST", "application/json", `{"text":""}`, 400, "empty_text"},
		{"AC7 whitespace text", "POST", "application/json", `{"text":" \n\t "}`, 400, "empty_text"},
		{"AC8 too long", "POST", "application/json", `{"text":"` + strings.Repeat("a", maxTextLength+1) + `"}`, 400, "text_too_long"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeAnalyzer{result: contractResult}
			rec := serve(NewHandler(fake, quietLogger()), tc.method, "/analyze", tc.contentType, tc.body)
			if rec.Code != tc.status {
				t.Errorf("status = %d, want %d", rec.Code, tc.status)
			}
			if code := errorCode(t, rec); code != tc.code {
				t.Errorf("code = %q, want %q", code, tc.code)
			}
			if len(fake.calls) != 0 {
				t.Errorf("analyzer was called %d times", len(fake.calls))
			}
			if tc.status == 405 && rec.Header().Get("Allow") != "POST" {
				t.Errorf("Allow = %q, want POST", rec.Header().Get("Allow"))
			}
		})
	}
}

// FR-04 AC1–AC5
func TestAnalyzerFailures(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{fmt.Errorf("%w: deadline", analyzer.ErrTimeout), 504, "analyzer_timeout"},
		{fmt.Errorf("%w: refused", analyzer.ErrUnavailable), 502, "analyzer_unavailable"},
		{fmt.Errorf("%w: status 500", analyzer.ErrBadResponse), 502, "analyzer_error"},
		{errors.New("unexpected"), 502, "analyzer_error"},
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			fake := &fakeAnalyzer{err: tc.err}
			h := NewHandler(fake, quietLogger())
			rec := serve(h, "POST", "/analyze", "application/json", `{"text":"a"}`)
			if rec.Code != tc.status || errorCode(t, rec) != tc.code {
				t.Errorf("got %d %s, want %d %s", rec.Code, rec.Body, tc.status, tc.code)
			}
			if len(fake.calls) != 1 {
				t.Errorf("analyzer calls = %d, want 1 (no retries)", len(fake.calls))
			}
			// AC5: the handler keeps serving.
			fake.err, fake.result = nil, contractResult
			if rec := serve(h, "POST", "/analyze", "application/json", `{"text":"a"}`); rec.Code != 200 {
				t.Errorf("follow-up status = %d", rec.Code)
			}
		})
	}
}

// FR-06 AC1
func TestHealthz(t *testing.T) {
	fake := &fakeAnalyzer{}
	h := NewHandler(fake, quietLogger())
	rec := serve(h, "GET", "/healthz", "", "")
	if rec.Code != 200 || rec.Body.String() != `{"status":"ok"}`+"\n" {
		t.Errorf("got %d %s", rec.Code, rec.Body)
	}
	if len(fake.calls) != 0 {
		t.Error("healthz called the analyzer")
	}
	rec = serve(h, "POST", "/healthz", "", "")
	if rec.Code != 405 || rec.Header().Get("Allow") != "GET" || errorCode(t, rec) != "method_not_allowed" {
		t.Errorf("POST /healthz = %d %s", rec.Code, rec.Body)
	}
}

func TestUnknownPath(t *testing.T) {
	rec := serve(NewHandler(&fakeAnalyzer{}, quietLogger()), "GET", "/nope", "", "")
	if rec.Code != 404 || errorCode(t, rec) != "not_found" {
		t.Errorf("got %d %s", rec.Code, rec.Body)
	}
}

func TestPanicBecomesInternalError(t *testing.T) {
	rec := serve(NewHandler(&fakeAnalyzer{panic: true}, quietLogger()), "POST", "/analyze", "application/json", `{"text":"a"}`)
	if rec.Code != 500 || errorCode(t, rec) != "internal_error" {
		t.Errorf("got %d %s", rec.Code, rec.Body)
	}
}

// NFR-02: one log line per request, without the submitted text.
func TestRequestLogOmitsText(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	fake := &fakeAnalyzer{err: fmt.Errorf("%w: refused", analyzer.ErrUnavailable)}
	serve(NewHandler(fake, logger), "POST", "/analyze", "application/json", `{"text":"secret sentence"}`)

	if strings.Contains(logs.String(), "secret sentence") {
		t.Fatalf("log contains the text: %s", logs.String())
	}
	var line map[string]any
	last := strings.Split(strings.TrimSpace(logs.String()), "\n")
	if err := json.Unmarshal([]byte(last[len(last)-1]), &line); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"method", "path", "status", "duration_ms"} {
		if _, ok := line[key]; !ok {
			t.Errorf("request log line has no %q: %v", key, line)
		}
	}
	if line["status"] != float64(502) {
		t.Errorf("logged status = %v", line["status"])
	}
}
