package toolkit

type UnittestConfig struct {
	AuthToken string `json:"auth_token"`
	BaseURL   string `json:"base"` // "http://localhost:8000" example
}

// -- Test Spec

type TestSpec struct {
	BaseURL   string     `json:"base_url"`
	Endpoints []Endpoint `json:"endpoints"`
}

type Endpoint struct {
	Name   string `json:"name"`
	Method string `json:"method"`
	Tests  []Test `json:"tests"`
}

type Test struct {
	ID          string       `json:"id"`
	Request     RequestSpecs `json:"request"`
	Expectation Expectation  `json:"expect"`
}

type RequestSpecs struct {
	PathParams map[string]string `json:"path_params"`
	Query      map[string]string `json:"query"`
	Headers    map[string]string `json:"headers"`
	BodyJson   map[string]any    `json:"body_json"`
}

type Expectation struct {
	Status  []int `json:"status"`
	Content any   `json:"content"`
}

// -- Report

type UnittestReport struct { // !!!! \\\
	// Final Unittest Report Structure. This is the main exporting struct.
	Summary   UnittestSummary      `json:"summary"`
	Persisted bool                 `json:"persisted"`
	Results   []UnittestCaseResult `json:"results"`
}

type UnittestSummary struct {
	Total  int `json:"total"`
	Passed int `json:"passed"`
	Failed int `json:"failed"`
}

type UnittestCaseResult struct {
	Endpoint string `json:"endpoint"`
	Method   string `json:"method"`
	TestID   string `json:"test_id"`
	Passed   bool   `json:"passed"`
	Failure  string `json:"failure_type,omitempty"`
	Why      string `json:"why_failed,omitempty"`
	Error    string `json:"error,omitempty"`

	ExpectedStatus  []int `json:"expected_status,omitempty"`
	ExpectedContent any   `json:"expected_content,omitempty"`

	Status int    `json:"status"`
	Body   string `json:"body,omitempty"`

	LatencyMS int64 `json:"latency_ms"`
}
