package api

import (
	"net/url"
	"strings"
	"time"
)

type premiumEndpointRule struct {
	hostSlug string
	addOnUID string
}

var premiumEndpointRules = []premiumEndpointRule{
	{hostSlug: "api-hyperliquid-mainnet-orderbook", addOnUID: "gWKew2Qp"},
	{hostSlug: "api-hyperliquid-mainnet-grpc", addOnUID: "wQX7GZmK"},
	{hostSlug: "api-asset-hub-kusama-sidecar", addOnUID: "79OaqZmE"},
	{hostSlug: "api-asset-hub-polkadot-sidecar", addOnUID: "1Qp5KY9E"},
	{hostSlug: "api-assethub-polkadot-sidecar", addOnUID: "1Qp5KY9E"},
	{hostSlug: "api-assethub-kusama-sidecar", addOnUID: "79OaqZmE"},
	{hostSlug: "api-kusama-sidecar", addOnUID: "y9gpzRWM"},
	{hostSlug: "api-polkadot-sidecar", addOnUID: "E9L0oP9w"},
	{hostSlug: "api-centrifuge-sidecar", addOnUID: "rm06Nk9X"},
	{hostSlug: "api-kilt-sidecar", addOnUID: "z9MvOKW4"},
}

func ApplyPremiumEndpointLabels(chains []Chain, account *AccountInfo) []Chain {
	if len(chains) == 0 {
		return chains
	}

	rulesByHost := make(map[string]premiumEndpointRule, len(premiumEndpointRules))
	for _, rule := range premiumEndpointRules {
		rulesByHost[strings.ToLower(rule.hostSlug)] = rule
	}

	stateByHost := make(map[string]PremiumEndpointStateEntry)
	activeAddOnUIDs := make(map[string]struct{})
	now := time.Now().UTC()

	if account != nil {
		for _, entry := range account.PremiumEndpointState {
			host := strings.ToLower(strings.TrimSpace(entry.HostSlug))
			if host == "" {
				continue
			}
			stateByHost[host] = entry
		}

		if account.CurrentSubscription != nil {
			for _, addOn := range account.CurrentSubscription.SubscriptionAddOns {
				if !isSubscriptionAddOnActive(addOn.EndDate, now) {
					continue
				}
				uid := strings.ToLower(strings.TrimSpace(addOn.CanonicalAddOnUID()))
				if uid == "" {
					continue
				}
				activeAddOnUIDs[uid] = struct{}{}
			}
		}
	}

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
		parsed, err := url.Parse(raw)
		if err != nil || parsed.Host == "" {
			continue
		}
		host := strings.ToLower(parsed.Hostname())
		if host == "" {
			continue
		}
		return strings.Split(host, ".")[0]
	}
	return ""
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
