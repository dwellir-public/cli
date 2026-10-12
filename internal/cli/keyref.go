package cli

import (
	"errors"
	"strings"

	"github.com/dwellir-public/cli/internal/api"
)

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
	return getFormatter().Error("not_found", lookupErr.Error(), "Run 'dwellir keys list' to see key names.")
}

// resolveKeyFilter replaces a key name or masked value in a filter flag with
// the full key the API filters on. A selector that matches no current key
// passes through, so usage of a deleted key can still be filtered by value.
func resolveKeyFilter(client *api.Client, selector *string) error {
	value := strings.TrimSpace(*selector)
	if value == "" {
		return nil
	}
	key, err := api.NewKeysAPI(client).Find(value)
	var lookupErr *api.KeyLookupError
	if errors.As(err, &lookupErr) && !lookupErr.Ambiguous && !strings.Contains(value, "*") {
		key.APIKey = api.NewSecret(value)
	} else if err != nil {
		return formatKeyLookupError(err)
	}
	client.MaskInErrors(key.APIKey)
	*selector = key.APIKey.Reveal()
	return nil
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
