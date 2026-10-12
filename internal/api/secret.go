package api

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
)

// Secret holds an API key value. Printing and encoding always mask it, so a
// key reaches output, files or processes only through an explicit Reveal.
type Secret struct {
	value string
}

// NewSecret also records the value, so MaskKnownKeys can mask it in text the
// CLI does not control, such as an API error that repeats a request.
func NewSecret(value string) Secret {
	value = strings.TrimSpace(value)
	known.add(value)
	return Secret{value: value}
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
	return mask(s.value)
}

func mask(value string) string {
	if value == "" {
		return ""
	}
	if len(value) <= 12 {
		return "****"
	}
	return value[:4] + "****" + value[len(value)-4:]
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

// known holds every key value this process has decoded or resolved. Values
// of 12 characters or fewer are skipped: masking them everywhere would also
// hit ordinary words.
var known = &keySet{values: map[string]struct{}{}}

type keySet struct {
	mu     sync.Mutex
	values map[string]struct{}
}

func (k *keySet) add(value string) {
	if len(value) <= 12 {
		return
	}
	k.mu.Lock()
	k.values[value] = struct{}{}
	k.mu.Unlock()
}

// MaskKnownKeys masks every key value this process has seen.
func MaskKnownKeys(text string) string {
	known.mu.Lock()
	defer known.mu.Unlock()
	for value := range known.values {
		text = strings.ReplaceAll(text, value, mask(value))
	}
	return text
}

// Dwellir API keys are lowercase UUIDs.
var keyShaped = regexp.MustCompile(`\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b`)

// MaskKeyShapedValues masks every UUID in free text that can carry a key the
// CLI has never seen: error messages, echoed arguments and log messages.
func MaskKeyShapedValues(text string) string {
	return keyShaped.ReplaceAllStringFunc(text, mask)
}
