package reporter

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"synrax/toolkit"
)

// main exporting function
func RunUnittest(filepath string, config toolkit.UnittestConfig) error {

	// read given file path documentation
	docBytes, err := os.ReadFile(filepath)
	if err != nil {
		log.Printf("runner: documentation read failed file=%s error=%v", filepath, err)
		return err
	}
	documentation := string(docBytes)

	log.Printf("runner: documentation loaded bytes=%d", len(docBytes))

	// call spec API from server
	spec, err := toolkit.SynraxSpecCaller(documentation, config)
	if err != nil {
		log.Printf("runner: spec fetch failed error=%v", err)
		return err
	}
	log.Printf("runner: spec fetched endpoints=%d", len(spec.Endpoints))
	if len(spec.Endpoints) == 0 {
		return fmt.Errorf("received empty test spec from server")
	}
	// build documentation
	_, err = BuildReportFromDocumentation(spec, config)
	if err != nil {
		log.Printf("runner: report build failed error=%v", err)
		return err
	}
	log.Printf("runner: completed")
	return err
}

func BuildReportFromDocumentation(spec toolkit.TestSpec, cfg toolkit.UnittestConfig) (toolkit.UnittestReport, error) {
	log.Printf("runner.build: start base_from_spec=%s base_from_config=%s endpoints=%d", spec.BaseURL, cfg.BaseURL, len(spec.Endpoints))

	if spec.BaseURL == "" {
		spec.BaseURL = cfg.BaseURL
		log.Printf("runner.build: spec base empty; fallback to config base=%s", spec.BaseURL)
	}

	report := Run(spec, cfg) // run test with given test spec
	report.Persisted = false
	log.Printf("runner.build: test run complete total=%d passed=%d failed=%d", report.Summary.Total, report.Summary.Passed, report.Summary.Failed)

	// write report.json
	path, err := filepath.Abs("./report.json")
	if err != nil {
		log.Printf("runner.build: failed resolve report path error=%v", err)
		return toolkit.UnittestReport{}, err
	}
	if err := writeJSON(path, report); err != nil {
		log.Printf("runner.build: failed write report path=%s error=%v", path, err)
		return toolkit.UnittestReport{}, fmt.Errorf("persist report json: %w", err)
	}
	report.Persisted = true
	log.Printf("runner.build: report persisted path=%s", path)

	return report, nil
}

func writeJSON(path string, data any) error {
	log.Printf("runner.write_json: writing file=%s", path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("prepare output directory for %q: %w", path, err)
	}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json %q: %w", path, err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return fmt.Errorf("write json file %q: %w", path, err)
	}
	return nil
}

func stringsTrimOrDefault(value, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}
