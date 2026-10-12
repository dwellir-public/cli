package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dwellir-public/cli/internal/api"
	"github.com/dwellir-public/cli/internal/output"
)

// maskedKeysNotice goes with every output that used to print full keys, so
// an agent or a person learns how to use a key without printing it.
const maskedKeysNotice = "API keys are masked and are never printed. To use one, run " +
	"'dwellir keys copy <name>' to copy it to the clipboard, or 'dwellir keys env <name>' to write DWELLIR_API_KEY to .env."

const endpointKeyNotice = "URLs contain a <key> placeholder. Run 'dwellir keys env <name>' to write DWELLIR_API_KEY to .env " +
	"and replace <key> with it at runtime, or run 'dwellir project setup --chain <chain> --network <network> --key-name <name>' " +
	"to write full URLs to .env."

// withNotice attaches notice when show is true.
func withNotice(data interface{}, show bool, notice string) interface{} {
	if !show {
		return data
	}
	return output.Notice{Data: data, Text: notice}
}

// keyNameArg accepts exactly one key selector and explains how to find one.
func keyNameArg(example string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 {
			return nil
		}
		message := "Missing required argument <name>."
		if len(args) > 1 {
			message = fmt.Sprintf("Expected one key name, got %d arguments.", len(args))
		}
		return getFormatter().Error("validation_error", message,
			"Pass a key name, a masked key such as 'abcd****wxyz', or a full key. "+
				"Run 'dwellir keys list' to see key names.\nExample: "+example)
	}
}

// findKey resolves a key name, masked value or full value to one key, and
// renders the CLI error when that fails.
func findKey(client *api.Client, selector string) (api.APIKey, error) {
	key, err := api.NewKeysAPI(client).Find(selector)
	if err != nil {
		return api.APIKey{}, formatKeyLookupError(err)
	}
	return key, nil
}

func formatKeyLookupError(err error) error {
	var lookupErr *api.KeyLookupError
	if !errors.As(err, &lookupErr) {
		return formatCommandError(err)
	}
	if lookupErr.Ambiguous {
		return getFormatter().Error(
			"validation_error",
			lookupErr.Error(),
			"Matching keys: "+strings.Join(lookupErr.Candidates, ", ")+
				". Select one by its masked value, for example: dwellir keys update 'abcd****wxyz' --name new-name",
		)
	}
	return getFormatter().Error("not_found", lookupErr.Error(),
		"Pass a key name, a masked key such as 'abcd****wxyz', or a full key. Run 'dwellir keys list' to see key names.")
}

// keyFilterValue turns a key name or masked value from a filter flag into
// the full key the API filters on. A selector that matches no current key
// passes through, so usage of a deleted key can still be filtered by value.
func keyFilterValue(client *api.Client, selector string) (string, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return "", nil
	}
	key, err := api.NewKeysAPI(client).Find(selector)
	var lookupErr *api.KeyLookupError
	if errors.As(err, &lookupErr) && !lookupErr.Ambiguous && !strings.Contains(selector, "*") {
		return api.NewSecret(selector).Reveal(), nil
	}
	if err != nil {
		return "", formatKeyLookupError(err)
	}
	return key.APIKey.Reveal(), nil
}

// keyNames maps each key to its name. Log rows carry only the key, and a
// masked key alone is hard to recognise. The names are best effort: when the
// key list fails, rows still identify keys by masked value.
func keyNames(client *api.Client) map[api.Secret]string {
	keys, err := api.NewKeysAPI(client).List()
	if err != nil {
		return nil
	}
	names := make(map[api.Secret]string, len(keys))
	for _, key := range keys {
		names[key.APIKey] = key.Name
	}
	return names
}

func nameErrorLogs(client *api.Client, rows []api.ErrorLog) []api.ErrorLog {
	if len(rows) == 0 {
		return rows
	}
	names := keyNames(client)
	for i := range rows {
		rows[i].APIKeyName = names[rows[i].APIKey]
	}
	return rows
}

func nameKeyFacets(client *api.Client, facets *api.ErrorFacets) *api.ErrorFacets {
	if facets == nil || len(facets.APIKeys) == 0 {
		return facets
	}
	names := keyNames(client)
	for i := range facets.APIKeys {
		facets.APIKeys[i].Name = names[facets.APIKeys[i].Value]
	}
	return facets
}
