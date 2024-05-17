package e2e

import (
	"fmt"

	"os"

	"github.com/onsi/ginkgo/types"
)

type CustomReporter struct {
	OutputFile *os.File
}

func (r *CustomReporter) SpecDidComplete(specSummary *types.SpecSummary) {
	if specSummary.Failed() {
		fmt.Fprintf(r.OutputFile, "FAIL: %s\n", specSummary.ComponentTexts[0])
	} else {
		fmt.Fprintf(r.OutputFile, "PASS: %s\n", specSummary.ComponentTexts[0])
	}
}

func NewCustomReporter(outputFilePath string) (*CustomReporter, error) {
	outputFile, err := os.OpenFile(outputFilePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}
	return &CustomReporter{OutputFile: outputFile}, nil
}

func (r *CustomReporter) Close() error {
	return r.OutputFile.Close()
}
