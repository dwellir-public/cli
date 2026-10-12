package output

import (
	"errors"
	"io"
	"time"

	"github.com/dwellir-public/cli/internal/api"
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
	Command    string `json:"command"`
	Timestamp  string `json:"timestamp"`
	Profile    string `json:"profile,omitempty"`
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    *bool  `json:"has_more,omitempty"`
	Notice     string `json:"notice,omitempty"`
}

// Notice attaches a hint to command data, such as how to use a masked key.
// JSON and TOON report it in meta.notice; human output prints it last.
type Notice struct {
	Data interface{}
	Text string
}

// Page is list data with a pagination cursor. Formatters print Items as the
// data and report the cursor in meta, so scripts can read every page.
type Page struct {
	Items      interface{}
	NextCursor string
	HasMore    bool
}

func successResponse(command string, data interface{}) Response {
	meta := &Meta{
		Command:   command,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	if notice, ok := data.(Notice); ok {
		data = notice.Data
		meta.Notice = notice.Text
	}
	if page, ok := data.(Page); ok {
		data = page.Items
		hasMore := page.HasMore
		meta.HasMore = &hasMore
		meta.NextCursor = page.NextCursor
	}
	return Response{OK: true, Data: data, Meta: meta}
}

// Formatter defines how CLI output is rendered.
type Formatter interface {
	Success(command string, data interface{}) error
	Error(code string, message string, help string) error
	Write(data interface{}) error
}

// RenderedError indicates an error message has already been rendered to the user.
type RenderedError struct {
	Code    string
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
// It backs up api.Secret: output masks every key value the process has seen,
// and errors also mask key-shaped values, such as a key passed as an argument.
func New(format string, w io.Writer) Formatter {
	w = keyMaskingWriter{w: w}
	if format == "json" {
		return keyMaskingErrors{NewJSONFormatter(w)}
	}
	if format == "toon" {
		return keyMaskingErrors{NewTOONFormatter(w)}
	}
	return keyMaskingErrors{NewHumanFormatter(w)}
}

type keyMaskingWriter struct {
	w io.Writer
}

func (m keyMaskingWriter) Write(p []byte) (int, error) {
	if _, err := io.WriteString(m.w, api.MaskKnownKeys(string(p))); err != nil {
		return 0, err
	}
	return len(p), nil
}

type keyMaskingErrors struct {
	Formatter
}

func (m keyMaskingErrors) Error(code string, message string, help string) error {
	return m.Formatter.Error(code, api.MaskKeyShapedValues(message), api.MaskKeyShapedValues(help))
}
