package output

import (
	"encoding/json"
	"io"
	"time"

	toon "github.com/toon-format/toon-go"
)

type TOONFormatter struct {
	w io.Writer
}

func NewTOONFormatter(w io.Writer) *TOONFormatter {
	return &TOONFormatter{w: w}
}

func (f *TOONFormatter) Success(command string, data interface{}) error {
	resp := Response{
		OK:   true,
		Data: data,
		Meta: &Meta{
			Command:   command,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
	}
	return f.encode(resp)
}

func (f *TOONFormatter) Error(code string, message string, help string) error {
	resp := Response{
		OK: false,
		Error: &ErrorBody{
			Code:    code,
			Message: message,
			Help:    help,
		},
	}
	if err := f.encode(resp); err != nil {
		return err
	}
	return &RenderedError{Message: message}
}

func (f *TOONFormatter) Write(data interface{}) error {
	return f.encode(data)
}

func (f *TOONFormatter) encode(v interface{}) error {
	normalized, err := normalizeForTOON(v)
	if err != nil {
		return err
	}
	b, err := toon.Marshal(normalized)
	if err != nil {
		return err
	}
	if len(b) == 0 || b[len(b)-1] != '\n' {
		b = append(b, '\n')
	}
	_, err = f.w.Write(b)
	return err
}

func normalizeForTOON(v interface{}) (interface{}, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out interface{}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}
