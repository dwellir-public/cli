package output

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"text/tabwriter"
)

type HumanFormatter struct {
	w io.Writer
}

func NewHumanFormatter(w io.Writer) *HumanFormatter {
	return &HumanFormatter{w: w}
}

func (f *HumanFormatter) Success(command string, data interface{}) error {
	return f.Write(data)
}

func (f *HumanFormatter) Error(code string, message string, help string) error {
	if _, err := fmt.Fprintf(f.w, "Error: %s\n", message); err != nil {
		return err
	}
	if help != "" {
		if _, err := fmt.Fprintf(f.w, "\n%s\n", help); err != nil {
			return err
		}
	}
	return errors.New(message)
}

func (f *HumanFormatter) Write(data interface{}) error {
	switch v := data.(type) {
	case []map[string]interface{}:
		return f.writeTable(v)
	case map[string]interface{}:
		return f.writeKeyValue(v)
	default:
		enc := json.NewEncoder(f.w)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	}
}

func (f *HumanFormatter) writeTable(rows []map[string]interface{}) error {
	if len(rows) == 0 {
		fmt.Fprintln(f.w, "No results.")
		return nil
	}
	tw := tabwriter.NewWriter(f.w, 0, 0, 2, ' ', 0)
	var headers []string
	for k := range rows[0] {
		headers = append(headers, k)
	}
	for _, h := range headers {
		fmt.Fprintf(tw, "%s\t", h)
	}
	fmt.Fprintln(tw)
	for _, row := range rows {
		for _, h := range headers {
			fmt.Fprintf(tw, "%v\t", row[h])
		}
		fmt.Fprintln(tw)
	}
	return tw.Flush()
}

func (f *HumanFormatter) writeKeyValue(data map[string]interface{}) error {
	tw := tabwriter.NewWriter(f.w, 0, 0, 2, ' ', 0)
	for k, v := range data {
		fmt.Fprintf(tw, "%s:\t%v\n", k, v)
	}
	return tw.Flush()
}
