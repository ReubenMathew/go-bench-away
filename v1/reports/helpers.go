package reports

import (
	"bytes"
	"fmt"
	"regexp"

	"github.com/montanaflynn/stats"
	"github.com/mprimi/go-bench-away/v1/core"
	"golang.org/x/perf/benchstat"
)

func loadJobAndResults(client JobRecordClient, jobId string) (*core.JobRecord, []byte, error) {
	job, _, err := client.LoadJob(jobId)
	if err != nil {
		return nil, nil, err
	}

	if job.Status != core.Succeeded && job.Status != core.Failed {
		return nil, nil, fmt.Errorf("Job %s status is %v", job.Id, job.Status)
	}

	fmt.Printf("Loading job %s\n", jobId)
	const initialBufferSize = 1024
	buf := bytes.NewBuffer(make([]byte, 0, initialBufferSize))
	err = client.LoadResultsArtifact(job, buf)
	if err != nil {
		return nil, nil, err
	}

	return job, buf.Bytes(), nil
}

// Count the unique string in the slice
func countUnique(elements []string) int {
	set := make(map[string]struct{}, len(elements))
	for _, element := range elements {
		set[element] = struct{}{}
	}
	return len(set)
}

// Multiple jobs may use the same GitRef (e.g. when comparing two versions of go)
// This makes graphs and table hard to read, since the same ref appears.
// Try to compose a minimum label for each job that makes it unique
func createJobLabels(jobs []*core.JobRecord) []string {

	containsDuplicates := func(labels []string) bool {
		m := make(map[string]struct{}, len(labels))
		for _, l := range labels {
			if _, present := m[l]; present {
				return true
			}
			m[l] = struct{}{}
		}
		return false
	}

	// Function that creates a label from a job
	type LabelFunc func(*core.JobRecord) string

	labelFunctions := []LabelFunc{
		// Try GitRef (or short SHA if ref is the SHA)
		func(job *core.JobRecord) string {
			if job.Parameters.GitRef == job.SHA {
				return job.SHA[0:7]
			}
			return job.Parameters.GitRef
		},
		// Try GitRef + SHA (or just SHA if the GitRef is the SHA)
		func(job *core.JobRecord) string {
			if job.Parameters.GitRef == job.SHA {
				return job.SHA[0:7]
			}
			return fmt.Sprintf("%s [%s]", job.Parameters.GitRef, job.SHA[0:7])
		},
		// Try GitRef + Go version
		func(job *core.JobRecord) string { return fmt.Sprintf("%s [%s]", job.Parameters.GitRef, job.GoVersion) },
		// Last resort, use job ID
		func(job *core.JobRecord) string { return job.Id },
	}

	for _, f := range labelFunctions {
		labels := make([]string, len(jobs))
		for i, job := range jobs {
			labels[i] = f(job)
		}
		if !containsDuplicates(labels) {
			return labels
		}
	}

	panic("Could not construct a set of unique labels")
}

// Return a new unique name for a chart div
var chartCounter int

func uniqueChartName() string {
	chartCounter += 1
	return fmt.Sprintf("chart_%d", chartCounter)
}

func resetChartId() {
	//TODO this is a ugly hack necessary for creating deterministic graphs in tests, find a better way
	chartCounter = 0
}

func valueDeviationAndScaledString(m *benchstat.Metrics) (float64, float64, string) {
	if len(m.RValues) == 0 {
		return 0, 0, "no data"
	}
	mean := m.Mean
	scaler := benchstat.NewScaler(mean, m.Unit)
	centile, err := stats.Percentile(m.RValues, kCentilePercent)
	if err != nil {
		panic(fmt.Sprintf("Failed to calculate percentile for %T %+v: %v", m, m, err))
	}
	deviation := centile - mean
	scaledString := fmt.Sprintf("%s ± %s", scaler(mean), scaler(deviation))
	return mean, deviation, scaledString
}

func filterByBenchmarkName(inputRows []*benchstat.Row, filter *regexp.Regexp) []*benchstat.Row {
	if filter == nil {
		return inputRows
	}

	outputRows := make([]*benchstat.Row, 0, len(inputRows))
	for _, row := range inputRows {
		if filter.MatchString(row.Benchmark) {
			outputRows = append(outputRows, row)
		}
	}
	return outputRows
}

func compileFilter(filterExpr string) *regexp.Regexp {
	if filterExpr == "" {
		return nil
	}
	return regexp.MustCompile(filterExpr)
}
