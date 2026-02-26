package toolkit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// This module calls our APIs to load the EndpointModule, Test Spec, and Application Config
// using internal tools by calling our server

// call test spec JSON
func SynraxSpecCaller(docs string, cfg UnittestConfig) (TestSpec, error) {
	BASE := os.Getenv("SYNRAX_API_BASE_URL")
	if strings.TrimSpace(BASE) == "" {
		return TestSpec{}, fmt.Errorf("API_BASE_URL is empty")
	}

	if strings.TrimSpace(cfg.BaseURL) == "" {
		return TestSpec{}, fmt.Errorf("config.base is empty (db and env fallback missing)")
	}
	if !isAbsoluteURL(cfg.BaseURL) {
		return TestSpec{}, fmt.Errorf("config.base must be an absolute URL, got=%q", cfg.BaseURL)
	}

	URL := fmt.Sprintf("%s/ai/test_spec", BASE)
	log.Printf("toolkit.spec: start url=%s docs_bytes=%d config_base=%s auth_token_present=%t", URL, len(docs), cfg.BaseURL, strings.TrimSpace(cfg.AuthToken) != "")

	payload := struct {
		Documentation string         `json:"documentation"`
		Config        UnittestConfig `json:"config"`
	}{
		Documentation: docs,
		Config:        cfg,
	}

	resp, body, err := postJSON(URL, payload)
	if err != nil {
		log.Printf("toolkit.spec: request failed url=%s error=%v", URL, err)
		return TestSpec{}, err
	}
	log.Printf("toolkit.spec: response status=%d", resp.StatusCode)

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		log.Printf("toolkit.spec: non-2xx status=%d body=%s", resp.StatusCode, truncateForLog(body, 500))
		return TestSpec{}, fmt.Errorf("test_spec request failed with status=%d body=%s", resp.StatusCode, truncateForLog(body, 500))
	}

	spec, err := decodeSpecBody(body)
	if err != nil {
		log.Printf("toolkit.spec: decode failed error=%v body=%s", err, truncateForLog(body, 300))
		return TestSpec{}, err
	}
	log.Printf("toolkit.spec: response body bytes=%d", len(body))
	if len(spec.Endpoints) == 0 {
		log.Printf("toolkit.spec: empty endpoints response body=%s", truncateForLog(body, 2000))
	}
	log.Printf("toolkit.spec: decoded endpoints=%d", len(spec.Endpoints))

	return spec, nil
}

// DB interaction to check on user's config
func SynraxConfigCaller(repo_id string) (UnittestConfig, error) {
	// we need to fetch config that our program requires to run internally

	BASE := os.Getenv("SYNRAX_API_BASE_URL")

	URL := fmt.Sprintf("%s/db/read?table=global_config", BASE)
	log.Printf("toolkit.config: start url=%s repo_id=%s", URL, repo_id)

	payload := struct {
		Filter map[string]string `json:"filter"`
	}{
		Filter: map[string]string{
			"id": repo_id,
		},
	}

	resp, body, err := postJSON(URL, payload)
	if err != nil {
		log.Printf("toolkit.config: request failed url=%s error=%v", URL, err)
		return UnittestConfig{}, err
	}
	log.Printf("toolkit.config: response status=%d", resp.StatusCode)

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		log.Printf("toolkit.config: non-2xx status=%d body=%s", resp.StatusCode, truncateForLog(body, 500))
		return UnittestConfig{}, fmt.Errorf("config request failed with status=%d body=%s", resp.StatusCode, truncateForLog(body, 500))
	}

	config, err := decodeConfigBody(body)
	if err != nil {
		log.Printf("toolkit.config: decode failed error=%v body=%s", err, truncateForLog(body, 500))
		return UnittestConfig{}, err
	}

	log.Printf("toolkit.config: decoded base=%s auth_token_present=%t", config.BaseURL, config.AuthToken != "")

	return config, nil
}

// Authenticate OIDC

func SynraxOIDCCaller(repo_id string, OIDCtoken string) (bool, error) {

	BASE := os.Getenv("SYNRAX_API_BASE_URL")
	URL := fmt.Sprintf("%s/github/oidc_validate?oidc_token=%s&repo_id=%s", BASE, OIDCtoken, repo_id)

	resp, err := http.Get(URL)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return false, fmt.Errorf("Unexpected status code passed: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	var oidc OIDCResp
	if err := json.Unmarshal(body, &oidc); err != nil {
		return false, err
	}

	if oidc.Status == "failure" { // case: OIDC Token is not valid or system failed
		return false, fmt.Errorf("%s", oidc.Reason)
	}

	return true, nil // case: OIDC Token is valid
}

// ---------- helpers

func decodeConfigBody(body []byte) (UnittestConfig, error) {
	var direct UnittestConfig
	if err := json.Unmarshal(body, &direct); err == nil {
		if strings.TrimSpace(direct.BaseURL) != "" || strings.TrimSpace(direct.AuthToken) != "" {
			return direct, nil
		}
	}

	var anyBody any
	if err := json.Unmarshal(body, &anyBody); err != nil {
		return UnittestConfig{}, err
	}
	rawMap, ok := findMapWithConfigKeys(anyBody)
	if !ok {
		return UnittestConfig{}, fmt.Errorf("could not find config object with base/auth_token fields")
	}

	raw, err := json.Marshal(rawMap)
	if err != nil {
		return UnittestConfig{}, err
	}

	var cfg UnittestConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return UnittestConfig{}, err
	}
	return cfg, nil
}

func decodeSpecBody(body []byte) (TestSpec, error) {
	var direct TestSpec
	if err := json.Unmarshal(body, &direct); err == nil {
		if len(direct.Endpoints) > 0 || strings.TrimSpace(direct.BaseURL) != "" {
			return direct, nil
		}
	}

	var wrapper struct {
		Response json.RawMessage `json:"response"`
	}
	if err := json.Unmarshal(body, &wrapper); err == nil && len(wrapper.Response) > 0 {
		var nested TestSpec
		if err := json.Unmarshal(wrapper.Response, &nested); err == nil {
			return nested, nil
		}
	}

	return TestSpec{}, fmt.Errorf("could not find test spec payload (expected top-level or response wrapper)")
}

func findMapWithConfigKeys(v any) (map[string]any, bool) {
	switch t := v.(type) {
	case map[string]any:
		_, hasBase := t["base"]
		_, hasAuth := t["auth_token"]
		if hasBase || hasAuth {
			return t, true
		}
		for _, child := range t {
			if m, ok := findMapWithConfigKeys(child); ok {
				return m, true
			}
		}
	case []any:
		for _, child := range t {
			if m, ok := findMapWithConfigKeys(child); ok {
				return m, true
			}
		}
	}
	return nil, false
}

func isAbsoluteURL(s string) bool {
	u, err := url.Parse(strings.TrimSpace(s))
	if err != nil {
		return false
	}
	return u.Scheme != "" && u.Host != ""
}

func truncateForLog(body []byte, max int) string {
	if len(body) <= max {
		return string(body)
	}
	return string(body[:max]) + "..."
}

func postJSON(url string, payload any) (*http.Response, []byte, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal payload: %w", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewReader(raw))
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	return resp, body, nil
}
