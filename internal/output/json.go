package output

import (
	"encoding/json"
	"io"
)

type JSONFormatter struct {
	w io.Writer
}

func NewJSONFormatter(w io.Writer) *JSONFormatter {
	return &JSONFormatter{w: w}
}

func (f *JSONFormatter) Success(command string, data interface{}) error {
	return f.encode(successResponse(command, data))
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
	return &RenderedError{Code: code, Message: message}
}

func (f *JSONFormatter) Write(data interface{}) error {
	return f.encode(data)
}

func (f *JSONFormatter) encode(v interface{}) error {
	enc := json.NewEncoder(f.w)
	// Keep <name> and & readable: hints hold commands that agents copy.
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}
