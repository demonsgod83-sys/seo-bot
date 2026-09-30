package report

import (
	"encoding/json"
	"fmt"
)

// JSONRenderer serializes the canonical ReportData into pretty JSON bytes.
type JSONRenderer struct{}

func NewJSONRenderer() *JSONRenderer {
	return &JSONRenderer{}
}

func (r *JSONRenderer) Render(data *ReportData) ([]byte, error) {
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to serialize report to JSON: %w", err)
	}
	return bytes, nil
}
