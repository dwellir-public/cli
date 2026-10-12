package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dwellir-public/cli/internal/api"
)

const maxFilterSuggestions = 3

// unknownFilterValue is a filter value that names no component.
type unknownFilterValue struct {
	Kind        string
	Value       string
	Suggestions []string
}

// findUnknownFilterValues returns every filter value missing from the component
// catalog, each with up to three close matches.
func findUnknownFilterValues(filter api.StatusFilter, components *api.StatusComponents) []unknownFilterValue {
	chains, networks, endpoints := componentFilterValues(components)
	unknown := []unknownFilterValue{}
	check := func(kind string, values, known []string) {
		for _, value := range values {
			if !containsString(known, value) {
				unknown = append(unknown, unknownFilterValue{Kind: kind, Value: value, Suggestions: suggestFilterValues(value, known)})
			}
		}
	}
	check("chain", filter.Chains, chains)
	check("network", filter.Networks, networks)
	check("endpoint", filter.Endpoints, endpoints)
	return unknown
}

func componentFilterValues(components *api.StatusComponents) (chains, networks, endpoints []string) {
	for _, chain := range components.Chains {
		chains = append(chains, strings.ToLower(chain.Slug))
		for _, network := range chain.Networks {
			networks = append(networks, strings.ToLower(network.Ref))
			for _, endpoint := range network.Endpoints {
				endpoints = append(endpoints, strings.ToLower(endpoint.FQDN))
			}
		}
	}
	return chains, networks, endpoints
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// unknownFilterError renders a validation error (exit 4) that names each
// unknown value and its suggestions.
func unknownFilterError(unknown []unknownFilterValue, mentionNoValidate bool) error {
	names := make([]string, 0, len(unknown))
	hints := []string{}
	for _, item := range unknown {
		names = append(names, fmt.Sprintf("%s %q", item.Kind, item.Value))
		if len(item.Suggestions) > 0 {
			hints = append(hints, fmt.Sprintf("Did you mean %s %s?", item.Kind, strings.Join(item.Suggestions, " or ")))
		}
	}
	message := "Unknown " + strings.Join(names, ", ") + "."
	hints = append(hints, "Run 'dwellir status components' to list valid chains, networks, and endpoints.")
	if mentionNoValidate {
		hints = append(hints, "Add --no-validate to skip this check.")
	}
	return statusFail("validation_error", message, strings.Join(hints, "\n"))
}

// suggestFilterValues ranks known values by closeness to value. A known value
// that contains value, or is contained in it, ranks first; then values within
// a small edit distance.
func suggestFilterValues(value string, known []string) []string {
	type candidate struct {
		value    string
		distance int
	}
	limit := len(value) / 3
	if limit < 2 {
		limit = 2
	}
	candidates := []candidate{}
	for _, item := range known {
		distance := levenshtein(value, item)
		if strings.Contains(item, value) || strings.Contains(value, item) {
			distance = 0
		}
		if distance <= limit {
			candidates = append(candidates, candidate{value: item, distance: distance})
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].distance != candidates[j].distance {
			return candidates[i].distance < candidates[j].distance
		}
		return candidates[i].value < candidates[j].value
	})
	out := []string{}
	for _, item := range candidates {
		if len(out) == maxFilterSuggestions {
			break
		}
		out = append(out, item.value)
	}
	return out
}

func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	previous := make([]int, len(rb)+1)
	for j := range previous {
		previous[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		current := make([]int, len(rb)+1)
		current[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			current[j] = min(previous[j]+1, current[j-1]+1, previous[j-1]+cost)
		}
		previous = current
	}
	return previous[len(rb)]
}
