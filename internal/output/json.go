package output

import (
	"encoding/json"
	"io"
	"time"
)

type JSONFormatter struct {
	w io.Writer
}

func NewJSONFormatter(w io.Writer) *JSONFormatter {
	return &JSONFormatter{w: w}
}

func (f *JSONFormatter) Success(command string, data interface{}) error {
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

func (f *JSONFormatter) Error(code string, message string, help string) error {
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

func (f *JSONFormatter) Write(data interface{}) error {
	return f.encode(data)
}

func (f *JSONFormatter) encode(v interface{}) error {
	enc := json.NewEncoder(f.w)
	return enc.Encode(v)
}
