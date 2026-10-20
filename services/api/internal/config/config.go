// Package config reads the API's runtime configuration from the environment
// (specs/requirements.md FR-07).
package config

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultPort            = "8080"
	defaultAnalyzerTimeout = 10 * time.Second
)

// Config is the validated runtime configuration of the API.
type Config struct {
	Port            string
	AnalyzerURL     string // absolute http(s) URL without a trailing slash
	AnalyzerTimeout time.Duration
}

// Load builds a Config from getenv (os.Getenv in production).
func Load(getenv func(string) string) (Config, error) {
	cfg := Config{Port: defaultPort, AnalyzerTimeout: defaultAnalyzerTimeout}

	if port := getenv("PORT"); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return Config{}, fmt.Errorf("PORT must be a number between 1 and 65535, got %q", port)
		}
		cfg.Port = port
	}

	raw := getenv("ANALYZER_URL")
	if raw == "" {
		return Config{}, fmt.Errorf("ANALYZER_URL is required (e.g. http://localhost:8081)")
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Config{}, fmt.Errorf("ANALYZER_URL must be an absolute http or https URL, got %q", raw)
	}
	cfg.AnalyzerURL = strings.TrimRight(raw, "/")

	if t := getenv("ANALYZER_TIMEOUT"); t != "" {
		d, err := time.ParseDuration(t)
		if err != nil || d <= 0 {
			return Config{}, fmt.Errorf("ANALYZER_TIMEOUT must be a positive Go duration such as 5s, got %q", t)
		}
		cfg.AnalyzerTimeout = d
	}
	return cfg, nil
}
