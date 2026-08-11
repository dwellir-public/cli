package api

import (
	"net/url"
	"strconv"
	"strings"
	"time"
)

type premiumEndpointRule struct {
	hostSlug  string
	chainName string
	addOnUID  string
}

// premiumEndpointRules mirrors frontend/src/config/locked-endpoints.ts. Neither
// copy is served by an API yet, so they must be kept in sync by hand.
var premiumEndpointRules = []premiumEndpointRule{
	{hostSlug: "api-hyperliquid-mainnet-orderbook", chainName: "Hyperliquid Orderbook", addOnUID: "gWKew2Qp"},
	{hostSlug: "api-hyperliquid-testnet-orderbook", chainName: "Hyperliquid Orderbook Testnet", addOnUID: "amRyMMWJ"},
	{hostSlug: "api-hyperliquid-mainnet-grpc", chainName: "Hyperliquid gRPC Mainnet", addOnUID: "wQX7GZmK"},
	{hostSlug: "api-hyperliquid-testnet-grpc", chainName: "Hyperliquid gRPC Testnet", addOnUID: "pWrOoamn"},
	{hostSlug: "api-asset-hub-kusama-sidecar", chainName: "AssetHub Kusama Sidecar", addOnUID: "79OaqZmE"},
	{hostSlug: "api-asset-hub-polkadot-sidecar", chainName: "AssetHub Polkadot Sidecar", addOnUID: "1Qp5KY9E"},
	{hostSlug: "api-assethub-polkadot-sidecar", chainName: "AssetHub Polkadot Sidecar", addOnUID: "1Qp5KY9E"},
	{hostSlug: "api-assethub-kusama-sidecar", chainName: "AssetHub Kusama Sidecar", addOnUID: "79OaqZmE"},
	{hostSlug: "api-kusama-sidecar", chainName: "Kusama Sidecar", addOnUID: "y9gpzRWM"},
	{hostSlug: "api-polkadot-sidecar", chainName: "Polkadot Sidecar", addOnUID: "E9L0oP9w"},
	{hostSlug: "api-centrifuge-sidecar", chainName: "Centrifuge Sidecar", addOnUID: "rm06Nk9X"},
	{hostSlug: "api-kilt-sidecar", chainName: "KILT Sidecar", addOnUID: "z9MvOKW4"},
}

func ApplyPremiumEndpointLabels(chains []Chain, account *AccountInfo) []Chain {
	if len(chains) == 0 {
		return chains
	}

	rulesByHost := make(map[string]premiumEndpointRule, len(premiumEndpointRules))
	for _, rule := range premiumEndpointRules {
		rulesByHost[strings.ToLower(rule.hostSlug)] = rule
	}

	stateByHost, activeAddOnUIDs := accountPremiumState(account)
	now := time.Now().UTC()

	for chainIdx := range chains {
		for networkIdx := range chains[chainIdx].Networks {
			for nodeIdx := range chains[chainIdx].Networks[networkIdx].Nodes {
				node := &chains[chainIdx].Networks[networkIdx].Nodes[nodeIdx]
				hostSlug := endpointHostSlug(node.HTTPS, node.WSS)
				if hostSlug == "" {
					continue
				}

				rule, isPremium := rulesByHost[hostSlug]
				if !isPremium {
					continue
				}

				status, trialEndsAt := resolvePremiumStatus(rule, stateByHost[hostSlug], activeAddOnUIDs, now)
				node.Premium = true
				node.PremiumStatus = string(status)
				node.TrialEndsAt = trialEndsAt

			}
		}
	}

	return chains
}

func endpointHostSlug(httpsURL, wssURL string) string {
	for _, raw := range []string{httpsURL, wssURL} {
		if slug := NormalizeEndpointSlug(raw); slug != "" {
			return slug
		}
	}
	return ""
}

// NormalizeEndpointSlug turns a host slug, an FQDN, or a full endpoint URL into
// the host slug the platform keys premium-endpoint entitlements by.
//
// The rule is marly's: the first DNS label of the hostname, lowercased
// (pymarly/endpoint_entitlements.py:9-10, endpoint_fqdn_to_host_slug). All three
// input shapes are accepted because a user reading `endpoints list` has a URL
// on screen, not a slug.
func NormalizeEndpointSlug(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}

	if strings.Contains(value, "://") {
		parsed, err := url.Parse(value)
		if err != nil || parsed.Hostname() == "" {
			return ""
		}
		value = parsed.Hostname()
	}

	// A bare "host/path" or "host:443" never reaches url.Parse above, so trim
	// the path and port by hand.
	if idx := strings.IndexAny(value, "/?#"); idx >= 0 {
		value = value[:idx]
	}
	if idx := strings.LastIndex(value, ":"); idx >= 0 {
		if _, err := strconv.Atoi(value[idx+1:]); err == nil {
			value = value[:idx]
		}
	}

	value = strings.Trim(value, ".")
	if value == "" {
		return ""
	}
	return strings.ToLower(strings.Split(value, ".")[0])
}

func resolvePremiumStatus(
	rule premiumEndpointRule,
	entry PremiumEndpointStateEntry,
	activeAddOnUIDs map[string]struct{},
	now time.Time,
) (PremiumEndpointStatus, string) {
	if _, ok := activeAddOnUIDs[strings.ToLower(rule.addOnUID)]; ok {
		return PremiumStatusAddonActive, ""
	}

	status := normalizePremiumStatus(entry.Status)
	switch status {
	case PremiumStatusAddonActive:
		return PremiumStatusAddonActive, ""
	case PremiumStatusTrialActive:
		if trialEnded(entry.TrialEndsAt, now) {
			return PremiumStatusTrialExpired, entry.TrialEndsAt
		}
		return PremiumStatusTrialActive, entry.TrialEndsAt
	case PremiumStatusTrialExpired:
		return PremiumStatusTrialExpired, entry.TrialEndsAt
	default:
		return PremiumStatusLocked, ""
	}
}

func normalizePremiumStatus(status PremiumEndpointStatus) PremiumEndpointStatus {
	switch status {
	case PremiumStatusAddonActive, PremiumStatusTrialActive, PremiumStatusTrialExpired, PremiumStatusLocked:
		return status
	default:
		return PremiumStatusLocked
	}
}

func trialEnded(trialEndsAt string, now time.Time) bool {
	if strings.TrimSpace(trialEndsAt) == "" {
		return false
	}
	ts, err := time.Parse(time.RFC3339, trialEndsAt)
	if err != nil {
		return false
	}
	return !ts.After(now)
}

func isSubscriptionAddOnActive(endDate string, now time.Time) bool {
	endDate = strings.TrimSpace(endDate)
	if endDate == "" {
		return true
	}

	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05"} {
		if parsed, err := time.Parse(layout, endDate); err == nil {
			if parsed.Location() == time.Local {
				parsed = parsed.UTC()
			}
			return parsed.After(now)
		}
	}
	return true
}
