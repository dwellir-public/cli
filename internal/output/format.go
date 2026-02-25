package output

import "io"

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

// New returns a Formatter based on the format string ("json" or "human").
func New(format string, w io.Writer) Formatter {
	if format == "json" {
		return NewJSONFormatter(w)
	}
	return NewHumanFormatter(w)
}
