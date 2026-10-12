package output

import (
	"errors"
	"io"
	"strings"
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
// An error without help gets the default hint for its code.
func New(format string, w io.Writer) Formatter {
	w = NewKeyMaskingWriter(w)
	if format == "json" {
		return keyMaskingErrors{NewJSONFormatter(w)}
	}
	if format == "toon" {
		return keyMaskingErrors{NewTOONFormatter(w)}
	}
	return keyMaskingErrors{NewHumanFormatter(w)}
}

// NewKeyMaskingWriter masks every key value the process has seen.
func NewKeyMaskingWriter(w io.Writer) io.Writer {
	return keyMaskingWriter{w: w}
}

// NewDiagnosticWriter also masks key-shaped values. Use it for stderr, which
// carries only diagnostics, help and usage text.
func NewDiagnosticWriter(w io.Writer) io.Writer {
	return keyMaskingWriter{w: w, keyShaped: true}
}

type keyMaskingWriter struct {
	w         io.Writer
	keyShaped bool
}

func (m keyMaskingWriter) Write(p []byte) (int, error) {
	text := api.MaskKnownKeys(string(p))
	if m.keyShaped {
		text = api.MaskKeyShapedValues(text)
	}
	if _, err := io.WriteString(m.w, text); err != nil {
		return 0, err
	}
	return len(p), nil
}

type keyMaskingErrors struct {
	Formatter
}

func (m keyMaskingErrors) Error(code string, message string, help string) error {
	if strings.TrimSpace(help) == "" {
		help = DefaultHelp(code)
	}
	return m.Formatter.Error(code, api.MaskKeyShapedValues(message), api.MaskKeyShapedValues(help))
}

// DefaultHelp names the next step for an error code, so every error tells an
// agent or a person what to do instead.
func DefaultHelp(code string) string {
	switch code {
	case "not_authenticated":
		return "Run 'dwellir auth login', or set DWELLIR_TOKEN. Run 'dwellir doctor' to check the setup."
	case "forbidden":
		return "Run 'dwellir auth status' to check the account and organization, or 'dwellir auth login' to log in again."
	case "not_found":
		return "Check the name. List commands such as 'dwellir keys list' and 'dwellir endpoints list' show valid names."
	case "rate_limited":
		return "Wait a moment, then retry."
	case "validation_error":
		return "Run the command with --help to see valid arguments, flags and examples."
	case "configuration_error":
		return "Run 'dwellir doctor' to check the CLI configuration."
	default:
		return "Retry the command. If it fails again, run 'dwellir doctor' to check the CLI setup."
	}
}
