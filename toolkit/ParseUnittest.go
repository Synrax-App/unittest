package toolkit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

type GlobalData struct {
	Total  int
	Passed int
	Failed int
}

type EndpointData struct {
	Name           string
	Passed         string
	Method         string
	TestID         string
	ExpectedStatus []int
	Status         int
	Body           string
}

func ParseUnittest(resultPath string, report UnittestReport) error {
	p, err := filepath.Abs("./toolkit/templates")
	if err != nil {
		return err
	}

	file, err := os.Create(resultPath)
	if err != nil {
		return err
	}

	// case: There are no failed tests, so no explanation is needed
	if report.Summary.Failed == 0 {
		fmt.Println(report.Summary.Failed)
		statement := []byte("All API Endpoints Passed.")
		if _, err := file.Write(statement); err != nil {
			return err
		}
		return nil
	}

	if err := writeGlobalData(p, file, report); err != nil {
		return err
	}
	if err := writeEndpointFailure(p, file, report); err != nil {
		return err
	}

	return nil
}

func writeGlobalData(path string, file *os.File, report UnittestReport) error {
	global_tmp, err := template.ParseFiles(path + "/global.tpl")
	if err != nil {
		return err
	}

	globalData := GlobalData{
		Total:  report.Summary.Total,
		Passed: report.Summary.Passed,
		Failed: report.Summary.Failed,
	}

	if err := global_tmp.Execute(file, globalData); err != nil {
		return err
	}
	return nil
}

func writeEndpointFailure(path string, file *os.File, report UnittestReport) error {

	endpoint_tmp, err := template.ParseFiles(path + "/endpoint.tpl")
	if err != nil {
		return err
	}

	var failures []EndpointData
	for _, endpoint := range report.Results {
		if endpoint.Passed {
			continue
		}

		failures = append(failures, EndpointData{
			Name:           endpoint.Endpoint,
			Passed:         "False",
			Method:         endpoint.Method,
			TestID:         endpoint.TestID,
			ExpectedStatus: endpoint.ExpectedStatus,
			Status:         endpoint.Status,
			Body:           formatEndpointBody(endpoint.Body),
		})
	}

	for _, endpointData := range failures {
		if err := endpoint_tmp.Execute(file, endpointData); err != nil {
			return err
		}
	}

	return nil
}

func formatEndpointBody(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "```json\n{}\n```"
	}

	var parsed any
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return "```\n" + trimmed + "\n```"
	}

	formatted, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		return "```json\n" + trimmed + "\n```"
	}

	var buf bytes.Buffer
	buf.WriteString("```json\n")
	buf.Write(formatted)
	buf.WriteString("\n```")
	return buf.String()
}
