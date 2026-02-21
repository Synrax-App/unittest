package reporter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"synrax/toolkit"
)

func Run(spec toolkit.TestSpec, cfg toolkit.UnittestConfig) toolkit.UnittestReport {
	client := &http.Client{Timeout: 15 * time.Second}
	var rep toolkit.UnittestReport
	baseURL := spec.BaseURL
	if cfg.BaseURL != "" {
		baseURL = cfg.BaseURL
	}
	log.Printf("tester.run: start base_url=%s endpoints=%d", baseURL, len(spec.Endpoints))

	for _, ep := range spec.Endpoints {
		log.Printf("tester.run: endpoint name=%s method=%s tests=%d", ep.Name, ep.Method, len(ep.Tests))
		for _, tc := range ep.Tests {
			rep.Summary.Total++
			log.Printf("tester.run: case start endpoint=%s test_id=%s", ep.Name, tc.ID)
			res := runOne(client, baseURL, ep, tc, cfg)
			rep.Results = append(rep.Results, res)
			if res.Passed {
				rep.Summary.Passed++
			} else {
				rep.Summary.Failed++
			}
			log.Printf("tester.run: case done endpoint=%s test_id=%s passed=%t status=%d failure=%s", ep.Name, tc.ID, res.Passed, res.Status, res.Failure)
		}
	}
	log.Printf("tester.run: completed total=%d passed=%d failed=%d", rep.Summary.Total, rep.Summary.Passed, rep.Summary.Failed)
	return rep
}

func runOne(client *http.Client, baseURL string, ep toolkit.Endpoint, tc toolkit.Test, cfg toolkit.UnittestConfig) toolkit.UnittestCaseResult {
	cr := toolkit.UnittestCaseResult{
		Endpoint:        ep.Name,
		Method:          ep.Method,
		TestID:          tc.ID,
		ExpectedStatus:  append([]int(nil), tc.Expectation.Status...),
		ExpectedContent: tc.Expectation.Content,
	}

	fullURL, err := buildURL(baseURL, ep.Name, tc.Request.PathParams, tc.Request.Query)
	if err != nil {
		log.Printf("tester.run_one: build url failed endpoint=%s test_id=%s error=%v", ep.Name, tc.ID, err)
		cr.Passed = false
		cr.Failure = "request_build_error"
		cr.Why = "Failed to build request URL for this test case."
		cr.Error = "buildURL: " + err.Error()
		return cr
	}

	if limit, ok := parseRateLimitTestID(tc.ID); ok {
		if limit <= 0 {
			limit = 1
		}
		for i := 0; i < limit+1; i++ {
			status, raw, latency, runErr := executeRequest(client, ep, tc, cfg, fullURL)
			cr.LatencyMS += latency
			if runErr != nil {
				log.Printf("tester.run_one: request failed endpoint=%s test_id=%s error=%v", ep.Name, tc.ID, runErr)
				cr.Passed = false
				cr.Failure = "transport_error"
				cr.Why = "Request did not complete successfully."
				cr.Error = runErr.Error()
				return cr
			}
			cr.Status = status
			cr.Body = raw
		}
	} else {
		status, raw, latency, runErr := executeRequest(client, ep, tc, cfg, fullURL)
		cr.LatencyMS = latency
		if runErr != nil {
			log.Printf("tester.run_one: request failed endpoint=%s test_id=%s error=%v", ep.Name, tc.ID, runErr)
			cr.Passed = false
			cr.Failure = "transport_error"
			cr.Why = "Request did not complete successfully."
			cr.Error = runErr.Error()
			return cr
		}
		cr.Status = status
		cr.Body = raw
	}

	// ASSERT: status
	if !statusMatches(cr.Status, tc.Expectation.Status) {
		log.Printf("tester.run_one: status mismatch endpoint=%s test_id=%s got=%d expected=%v", ep.Name, tc.ID, cr.Status, tc.Expectation.Status)
		cr.Passed = false
		cr.Failure = "status_mismatch"
		cr.Why = buildStatusMismatchReason(tc.Expectation.Status, cr.Status, cr.Body)
		cr.Error = fmt.Sprintf("status mismatch (got=%d expected=%v)", cr.Status, tc.Expectation.Status)
		return cr
	}

	if tc.Expectation.Content != nil {
		var actual any
		if err := json.Unmarshal([]byte(cr.Body), &actual); err != nil {
			log.Printf("tester.run_one: response parse failed endpoint=%s test_id=%s error=%v", ep.Name, tc.ID, err)
			cr.Passed = false
			cr.Failure = "response_parse_error"
			cr.Why = "Expected structured content, but response body is not valid JSON."
			cr.Error = "response content is not valid JSON"
			return cr
		}
		if !contentMatches(actual, tc.Expectation.Content, isSuccessTest(tc.ID)) {
			log.Printf("tester.run_one: content mismatch endpoint=%s test_id=%s", ep.Name, tc.ID)
			cr.Passed = false
			cr.Failure = "content_mismatch"
			cr.Why = buildContentMismatchReason(tc.Expectation.Content, actual)
			cr.Error = "response content mismatch"
			return cr
		}
	}

	cr.Passed = true
	return cr
}

func executeRequest(client *http.Client, ep toolkit.Endpoint, tc toolkit.Test, cfg toolkit.UnittestConfig, fullURL string) (int, string, int64, error) {
	headers := cloneHeaders(tc.Request.Headers)
	if shouldInjectAuth(tc.ID, cfg.AuthToken) {
		if _, ok := headers["Authorization"]; !ok {
			headers["Authorization"] = "Bearer " + cfg.AuthToken
		}
	}
	headers["X-Unittest-Case"] = tc.ID

	var body io.Reader
	if ep.Method != "GET" && ep.Method != "DELETE" {
		if tc.Request.BodyJson != nil && len(tc.Request.BodyJson) > 0 {
			b, _ := json.Marshal(tc.Request.BodyJson)
			body = bytes.NewReader(b)
			if _, ok := headers["Content-Type"]; !ok && shouldInjectContentType(tc.ID) {
				headers["Content-Type"] = "application/json"
			}
		}
	}

	req, err := http.NewRequest(ep.Method, fullURL, body)
	if err != nil {
		return 0, "", 0, fmt.Errorf("NewRequest: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	start := time.Now()
	log.Printf("tester.execute: sending method=%s url=%s test_id=%s", ep.Method, fullURL, tc.ID)
	resp, err := client.Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return 0, "", latency, fmt.Errorf("Do: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	log.Printf("tester.execute: received method=%s url=%s test_id=%s status=%d latency_ms=%d", ep.Method, fullURL, tc.ID, resp.StatusCode, latency)
	return resp.StatusCode, string(raw), latency, nil
}

func shouldInjectAuth(testID string, token string) bool {
	if token == "" {
		return false
	}

	id := strings.ToLower(strings.TrimSpace(testID))
	if strings.Contains(id, "missing-auth") || strings.Contains(id, "missing_auth") {
		return false
	}
	if strings.Contains(id, "missing-required-header-authorization") {
		return false
	}
	if strings.Contains(id, "wrong-header-value-authorization") {
		return false
	}

	return true
}

func cloneHeaders(src map[string]string) map[string]string {
	out := make(map[string]string, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func buildURL(baseURL, endpoint string, pathParams, query map[string]string) (string, error) {
	u, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return "", err
	}

	path := endpoint
	for k, v := range pathParams {
		path = strings.ReplaceAll(path, "{"+k+"}", url.PathEscape(v))
	}
	u.Path = strings.TrimRight(u.Path, "/") + path

	q := u.Query()
	for k, v := range query {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()

	return u.String(), nil
}

func statusMatches(got int, allowed []int) bool {
	if len(allowed) == 0 {
		return got >= 200 && got <= 299
	}
	for _, s := range allowed {
		if got == s {
			return true
		}
	}
	return false
}

func parseRateLimitTestID(testID string) (int, bool) {
	id := strings.ToLower(strings.TrimSpace(testID))
	const prefix = "rate-limit-exceeded-"
	if !strings.HasPrefix(id, prefix) {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimPrefix(id, prefix))
	if err != nil {
		return 0, false
	}
	return n, true
}

func contentMatches(actual any, expected any, relaxNumbers bool) bool {
	switch exp := expected.(type) {
	case map[string]any:
		act, ok := actual.(map[string]any)
		if !ok {
			return false
		}
		for k, v := range exp {
			a, ok := act[k]
			if !ok {
				return false
			}
			if !contentMatches(a, v, relaxNumbers) {
				return false
			}
		}
		return true
	case []any:
		act, ok := actual.([]any)
		if !ok {
			return false
		}
		if len(exp) > len(act) {
			return false
		}
		for i := range exp {
			if !contentMatches(act[i], exp[i], relaxNumbers) {
				return false
			}
		}
		return true
	case string:
		if exp == "..." {
			if s, ok := actual.(string); ok {
				return strings.TrimSpace(s) != ""
			}
			return actual != nil
		}
		s, ok := actual.(string)
		return ok && s == exp
	case float64:
		if relaxNumbers {
			switch actual.(type) {
			case float64, int, int64:
				return true
			default:
				return false
			}
		}
		switch a := actual.(type) {
		case float64:
			return a == exp
		case int:
			return float64(a) == exp
		case int64:
			return float64(a) == exp
		default:
			return false
		}
	default:
		return actual == expected
	}
}

func shouldInjectContentType(testID string) bool {
	id := strings.ToLower(strings.TrimSpace(testID))
	if strings.Contains(id, "missing-required-header-content-type") {
		return false
	}
	return true
}

func isSuccessTest(testID string) bool {
	id := strings.ToLower(strings.TrimSpace(testID))
	return strings.Contains(id, "success-valid-request")
}

func buildStatusMismatchReason(expected []int, got int, rawBody string) string {
	base := fmt.Sprintf("Expected status in %v but received %d.", expected, got)
	if hint := genericErrorHint(rawBody); hint != "" {
		return base + " Response hint: " + hint
	}
	return base
}

func buildContentMismatchReason(expected any, actual any) string {
	if path, exp, act, ok := firstContentDifference("$", expected, actual); ok {
		return fmt.Sprintf("Response content mismatch at %s (expected=%s got=%s).", path, exp, act)
	}
	return "Response content did not match expected structure."
}

func firstContentDifference(path string, expected any, actual any) (string, string, string, bool) {
	switch exp := expected.(type) {
	case map[string]any:
		act, ok := actual.(map[string]any)
		if !ok {
			return path, compactForReport(expected), compactForReport(actual), true
		}
		for k, v := range exp {
			a, exists := act[k]
			if !exists {
				return path + "." + k, compactForReport(v), "<missing>", true
			}
			if p, e, av, diff := firstContentDifference(path+"."+k, v, a); diff {
				return p, e, av, true
			}
		}
		return "", "", "", false
	case []any:
		act, ok := actual.([]any)
		if !ok {
			return path, compactForReport(expected), compactForReport(actual), true
		}
		if len(act) < len(exp) {
			return path, fmt.Sprintf("len>=%d", len(exp)), fmt.Sprintf("len=%d", len(act)), true
		}
		for i := range exp {
			if p, e, av, diff := firstContentDifference(fmt.Sprintf("%s[%d]", path, i), exp[i], act[i]); diff {
				return p, e, av, true
			}
		}
		return "", "", "", false
	default:
		if !contentMatches(actual, expected, false) {
			return path, compactForReport(expected), compactForReport(actual), true
		}
		return "", "", "", false
	}
}

func genericErrorHint(rawBody string) string {
	if strings.TrimSpace(rawBody) == "" {
		return ""
	}
	var v any
	if err := json.Unmarshal([]byte(rawBody), &v); err != nil {
		return ""
	}
	return extractErrorHint(v)
}

func extractErrorHint(v any) string {
	switch obj := v.(type) {
	case map[string]any:
		priorityKeys := []string{"detail", "error", "errors", "message", "msg", "reason", "title"}
		for _, key := range priorityKeys {
			if val, ok := obj[key]; ok {
				return compactForReport(val)
			}
		}
		for _, val := range obj {
			if hint := extractErrorHint(val); hint != "" {
				return hint
			}
		}
	case []any:
		for _, item := range obj {
			if hint := extractErrorHint(item); hint != "" {
				return hint
			}
		}
	}
	return ""
}

func compactForReport(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}
