package report

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"patchbot/internal/model"
)

type failedWriter struct{}

func (failedWriter) Write([]byte) (int, error) { return 0, errors.New("output closed") }

func TestIncompleteEmptyReportDoesNotClaimCleanResult(t *testing.T) {
	var output bytes.Buffer
	r := model.Report{Status: "partial", Analysis: model.Analysis{Stages: map[string]model.Stage{}}}
	if err := Write(&output, "text", r); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Scan incomplete") || strings.Contains(output.String(), "No known vulnerabilities found") {
		t.Fatalf("misleading output: %s", &output)
	}
	for _, format := range []string{"text", "json"} {
		if err := Write(failedWriter{}, format, r); err == nil {
			t.Fatalf("ignored %s writer failure", format)
		}
	}
}
