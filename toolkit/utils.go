package toolkit

import "github.com/google/uuid"

func ReportMetrics(repoID string, report UnittestReport) (ReportMetric, error) {
	totalTests := report.Summary.Total

	passed := report.Summary.Passed
	failed := report.Summary.Failed

	passRate := ((passed - failed) / passed) * 100

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

	metrics := ReportMetric{
		ID:                   uuid.NewString(),
		RepoID:               repoID,
		TotalTests:           totalTests,
		Passed:               passed,
		Failed:               failed,
		SuccessRate:          float32(passRate),
		GetCounts:            methodCounts["GET"],
		PostCounts:           methodCounts["POST"],
		PutCounts:            methodCounts["PUT"],
		DeleteCounts:         methodCounts["DELETE"],
		UniqueEndpointsCount: len(keys),
		AverageLatency:       float32(sum / len(latencyArray)),
	}

	return metrics, nil
}
