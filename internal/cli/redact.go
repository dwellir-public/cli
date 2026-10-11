package cli

import (
	"strings"

	"github.com/dwellir-public/cli/internal/api"
)

// showSecrets turns off API key masking in usage and log output.
var showSecrets bool

const secretFlagHelp = "Show full API key values instead of masking them"

// maskAPIKey keeps the first and last four characters so a key stays
// recognisable without the output working as a credential.
func maskAPIKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" || showSecrets {
		return key
	}
	if len(key) <= 12 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}

func maskUsageHistory(rows []api.UsageHistory) []api.UsageHistory {
	for i := range rows {
		rows[i].APIKey = maskAPIKey(rows[i].APIKey)
	}
	return rows
}

func maskErrorLogs(rows []api.ErrorLog) []api.ErrorLog {
	for i := range rows {
		rows[i].APIKey = maskAPIKey(rows[i].APIKey)
	}
	return rows
}

func maskErrorFacets(facets *api.ErrorFacets) *api.ErrorFacets {
	if facets == nil {
		return nil
	}
	for i := range facets.APIKeys {
		facets.APIKeys[i].Value = maskAPIKey(facets.APIKeys[i].Value)
	}
	return facets
}
