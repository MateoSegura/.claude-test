package analysis

import (
	"encoding/json"
	"io"
)

// FormatJSON writes the report as indented JSON to w.
func FormatJSON(r *Report, w io.Writer) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte("\n"))
	return err
}
