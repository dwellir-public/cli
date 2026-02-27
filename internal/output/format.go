package output

import (
	"errors"
	"io"
)

// Response is the JSON envelope for all CLI output.
type Response struct {
	OK    bool        `json:"ok"`
	Data  interface{} `json:"data,omitempty"`
	Error *ErrorBody  `json:"error,omitempty"`
	Meta  *Meta       `json:"meta,omitempty"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Help    string `json:"help,omitempty"`
}

type Meta struct {
	Command   string `json:"command"`
	Timestamp string `json:"timestamp"`
	Profile   string `json:"profile,omitempty"`
}

// Formatter defines how CLI output is rendered.
type Formatter interface {
	Success(command string, data interface{}) error
	Error(code string, message string, help string) error
	Write(data interface{}) error
}

// RenderedError indicates an error message has already been rendered to the user.
type RenderedError struct {
	Message string
}

func (e *RenderedError) Error() string {
	return e.Message
}

// IsRenderedError reports whether err was already emitted by an output formatter.
func IsRenderedError(err error) bool {
	if err == nil {
		return false
	}
	var rendered *RenderedError
	return errors.As(err, &rendered)
}

// New returns a Formatter based on the format string ("json" or "human").
func New(format string, w io.Writer) Formatter {
	if format == "json" {
		return NewJSONFormatter(w)
	}
	return NewHumanFormatter(w)
}
