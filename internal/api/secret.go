package api

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Secret holds an API key value. Printing and encoding always mask it, so a
// key reaches output, files or processes only through an explicit Reveal.
type Secret struct {
	value string
}

func NewSecret(value string) Secret {
	return Secret{value: strings.TrimSpace(value)}
}

// Reveal returns the full key. Call it only to send the key to the API or to
// hand it to a destination the user chose: a file, the clipboard or a URL
// that is never printed.
func (s Secret) Reveal() string {
	return s.value
}

func (s Secret) IsZero() bool {
	return s.value == ""
}

// Masked keeps the first and last four characters so a key stays
// recognisable without working as a credential.
func (s Secret) Masked() string {
	if s.value == "" {
		return ""
	}
	if len(s.value) <= 12 {
		return "****"
	}
	return s.value[:4] + "****" + s.value[len(s.value)-4:]
}

func (s Secret) String() string {
	return s.Masked()
}

// Format masks the key for every fmt verb, including %v, %+v and %#v.
func (s Secret) Format(f fmt.State, verb rune) {
	if verb == 'q' {
		_, _ = fmt.Fprintf(f, "%q", s.Masked())
		return
	}
	_, _ = io.WriteString(f, s.Masked())
}

func (s Secret) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.Masked())
}

func (s *Secret) UnmarshalJSON(data []byte) error {
	var value *string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	if value == nil {
		*s = Secret{}
		return nil
	}
	*s = NewSecret(*value)
	return nil
}
