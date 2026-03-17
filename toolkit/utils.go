package toolkit

import "github.com/google/uuid"

func ReportMetrics(repoID string, report UnittestReport) (ReportMetric, error) {
	totalTests := report.Summary.Total

	passed := report.Summary.Passed
	failed := report.Summary.Failed

	var passRate float32 = 0.0
	if totalTests > 0 {
		passRate = (float32(passed) / float32(totalTests)) * 100.0
	}

	uniqueEndpoints := make(map[string]int)
	methodCounts := map[string]int{
		"GET":    0,
		"POST":   0,
		"PUT":    0,
		"DELETE": 0,
	}
	latencyArray := make([]int, 0, totalTests)

	for _, result := range report.Results {
		// endpoint name check
		endpointName := result.Endpoint
		if _, ok := uniqueEndpoints[endpointName]; !ok { // case not found in map
			uniqueEndpoints[endpointName] = 0
		} else {
			uniqueEndpoints[endpointName]++
		}
		// method counter
		testMethod := result.Method
		if _, ok := methodCounts[testMethod]; ok {
			methodCounts[testMethod]++
		}
		// latency
		testLatency := result.LatencyMS
		latencyArray = append(latencyArray, int(testLatency))
	}

	keys := make([]string, 0, len(uniqueEndpoints))
	for k := range uniqueEndpoints {
		keys = append(keys, k)
	}

	sum := 0
	for _, lat := range latencyArray {
		sum = sum + lat
	}

	var avgLatency float32 = 0.0
	if len(latencyArray) > 0 {
		avgLatency = float32(sum) / float32(len(latencyArray))
	}

	metrics := ReportMetric{
		ID:                   uuid.NewString(),
		RepoID:               repoID,
		TotalTests:           totalTests,
		Passed:               passed,
		Failed:               failed,
		SuccessRate:          passRate,
		GetCounts:            methodCounts["GET"],
		PostCounts:           methodCounts["POST"],
		PutCounts:            methodCounts["PUT"],
		DeleteCounts:         methodCounts["DELETE"],
		UniqueEndpointsCount: len(keys),
		AverageLatency:       avgLatency,
	}

	return metrics, nil
}
