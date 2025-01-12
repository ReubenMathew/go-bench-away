package reports

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/mprimi/go-bench-away/v1/core"
)

const (
	job1 = "067997a3-761e-475e-9559-f10d7400b835"
	job2 = "dd146049-0137-4ba0-89b1-0a2f8d0a2268"
	job3 = "e98b2caa-df6d-4f12-815c-431db896a9f5"
)

type mockClient struct {
}

func (m mockClient) LoadJob(jobId string) (*core.JobRecord, uint64, error) {
	recordPath := filepath.Join("testdata", fmt.Sprintf("%s.json", jobId))

	file, err := os.Open(recordPath)
	if err != nil {
		panic(err)
	}

	jr := &core.JobRecord{}

	err = json.NewDecoder(file).Decode(jr)
	if err != nil {
		panic(err)
	}

	return jr, 1, nil
}

func (m mockClient) LoadResultsArtifact(record *core.JobRecord, writer io.Writer) error {

	resultsPath := filepath.Join("testdata", fmt.Sprintf("%s_results.txt", record.Id))

	file, err := os.Open(resultsPath)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	_, err = io.Copy(writer, file)
	if err != nil {
		panic(err)
	}

	return nil
}

func TestWriteEmptyReport(t *testing.T) {
	resetChartId()
	cfg := ReportConfig{
		Title:   "Empty report",
		verbose: true,
	}

	writeReportAndCompareToExpected(
		t,
		[]string{job1, job2, job3},
		&cfg,
		"empty.html",
	)
}

func TestWriteTrendReport(t *testing.T) {
	resetChartId()
	cfg := &ReportConfig{
		Title:   "Trend report",
		verbose: true,
	}

	cfg.AddSections(
		JobsTable(),
		TrendChart("", TimeOp, ""),
		ResultsTable(TimeOp, "", true),
		TrendChart("", Speed, ""),
		ResultsTable(Speed, "", true),
	)

	writeReportAndCompareToExpected(
		t,
		[]string{job1, job2, job3},
		cfg,
		"trend.html",
	)
}

func TestWriteTrendReportFiltered(t *testing.T) {
	resetChartId()
	cfg := &ReportConfig{
		Title:   "Trend report",
		verbose: true,
	}

	filter := ".*JetStreamKV/.*/CAS"

	cfg.AddSections(
		JobsTable(),
		TrendChart("", TimeOp, filter),
		ResultsTable(TimeOp, filter, true),
		TrendChart("", Speed, filter),
		ResultsTable(Speed, filter, true),
	)

	writeReportAndCompareToExpected(
		t,
		[]string{job1, job2, job3},
		cfg,
		"trend_filtered.html",
	)
}

func TestWriteCompareNReport(t *testing.T) {
	resetChartId()
	cfg := &ReportConfig{
		Title:   "Comparative report",
		verbose: true,
	}

	filter := ""

	cfg.AddSections(
		JobsTable(),
		HorizontalBarChart("", TimeOp, filter),
		ResultsTable(TimeOp, filter, true),
		HorizontalBarChart("", Speed, filter),
		ResultsTable(Speed, filter, true),
	)

	writeReportAndCompareToExpected(
		t,
		[]string{job1, job2, job3},
		cfg,
		"compare_n.html",
	)
}

func TestWriteCompareReport(t *testing.T) {
	resetChartId()
	cfg := &ReportConfig{
		Title:   "Comparative report",
		verbose: true,
	}

	filter := ""

	cfg.AddSections(
		JobsTable(),
		HorizontalDeltaChart("", TimeOp, filter),
		ResultsDeltaTable(TimeOp, filter, true),
		HorizontalDeltaChart("", Speed, filter),
		ResultsDeltaTable(Speed, filter, true),
	)

	writeReportAndCompareToExpected(
		t,
		[]string{job1, job2},
		cfg,
		"compare.html",
	)
}

func TestWriteSingleReport(t *testing.T) {
	resetChartId()
	cfg := &ReportConfig{
		Title:   "Single results set report",
		verbose: true,
	}

	filter := ""

	cfg.AddSections(
		JobsTable(),
		HorizontalBoxChart("", TimeOp, filter),
		ResultsTable(TimeOp, filter, true),
		HorizontalBoxChart("", Speed, filter),
		ResultsTable(Speed, filter, true),
	)

	writeReportAndCompareToExpected(
		t,
		[]string{job1},
		cfg,
		"single.html",
	)
}

func writeReportAndCompareToExpected(t *testing.T, jobIds []string, reportConfig *ReportConfig, expectedReportName string) {
	var err error

	c := mockClient{}

	dataTable, err := CreateDataTable(c, jobIds...)
	if err != nil {
		t.Fatal(err)
	}

	if !dataTable.HasSpeed() {
		t.Fatalf("Expected speed data")
	}

	outputFilePath := filepath.Join(t.TempDir(), "report.html")
	file, err := os.Create(outputFilePath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	err = WriteReport(reportConfig, dataTable, file)
	if err != nil {
		t.Fatal(err)
	}

	assertReportEqual(t, outputFilePath, filepath.Join("testdata", expectedReportName))
}

func assertReportEqual(t *testing.T, reportPath string, expectedReportPath string) {

	reportFile, err := os.Open(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	defer reportFile.Close()

	expectedReportFile, err := os.Open(expectedReportPath)
	if err != nil {
		t.Fatal(err)
	}
	defer expectedReportFile.Close()

	reportDigest := md5.New()
	expectedReportDigest := md5.New()

	_, err = io.Copy(reportDigest, reportFile)
	if err != nil {
		t.Fatal(err)
	}

	_, err = io.Copy(expectedReportDigest, expectedReportFile)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(reportDigest.Sum(nil), expectedReportDigest.Sum(nil)) {
		// Set to true to copy the produced report over the expected report in the test data directory.
		// Useful to update the reports after a code change, assuming the new output is valid after being reviewed
		// via git diff.
		const overwriteTestData = false
		if overwriteTestData {
			err := os.Rename(reportPath, expectedReportPath)
			if err != nil {
				t.Log(err)
			}
		}
		t.Fatalf("Report %s does not match expected %s", reportPath, expectedReportPath)
	}
}
