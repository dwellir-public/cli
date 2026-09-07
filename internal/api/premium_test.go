package api

import (
	"encoding/json"
	"testing"
	"time"
)

func TestAccountInfoUnmarshalPremiumEndpointStateString(t *testing.T) {
	raw := `{
		"uid": "acct-123",
		"name": "Acme",
		"premiumEndpointState": "[{\"hostSlug\":\"api-hyperliquid-mainnet-orderbook\",\"status\":\"trial-active\",\"trialEndsAt\":\"2026-03-01T00:00:00Z\"}]",
		"currentSubscription": {
			"subscriptionAddOns": [
				{
					"uid":"L9P6xpnm",
					"name":"Hyperliquid Orderbook Service",
					"addOnUid":"gWKew2Qp",
					"endDate":"2030-01-01T00:00:00Z"
				}
			]
		}
	}`

	var info AccountInfo
	if err := json.Unmarshal([]byte(raw), &info); err != nil {
		t.Fatalf("failed to unmarshal account info: %v", err)
	}
	if info.UID != "acct-123" {
		t.Fatalf("expected account uid acct-123, got %q", info.UID)
	}

	if len(info.PremiumEndpointState) != 1 {
		t.Fatalf("expected 1 premium endpoint state entry, got %d", len(info.PremiumEndpointState))
	}
	if info.PremiumEndpointState[0].HostSlug != "api-hyperliquid-mainnet-orderbook" {
		t.Fatalf("unexpected host slug: %q", info.PremiumEndpointState[0].HostSlug)
	}
	if info.CurrentSubscription == nil || len(info.CurrentSubscription.SubscriptionAddOns) != 1 {
		t.Fatalf("expected current subscription add-ons to be parsed")
	}
	if info.CurrentSubscription.SubscriptionAddOns[0].AddOnUID != "gWKew2Qp" {
		t.Fatalf("expected addOnUid to be parsed, got %q", info.CurrentSubscription.SubscriptionAddOns[0].AddOnUID)
	}
}

func TestApplyPremiumEndpointLabelsLockedKeepsEndpointAndAddsLabel(t *testing.T) {
	chains := []Chain{
		{
			Name: "Hyperliquid HyperCore Orderbook",
			Networks: []Network{
				{
					Name: "Mainnet",
					Nodes: []Node{
						{
							HTTPS:    "https://api-hyperliquid-mainnet-orderbook.n.dwellir.com/<key>/ws",
							WSS:      "wss://api-hyperliquid-mainnet-orderbook.n.dwellir.com/<key>/ws",
							NodeType: NodeType{Name: "Full"},
						},
					},
				},
			},
		},
	}

	out := ApplyPremiumEndpointLabels(chains, &AccountInfo{})
	node := out[0].Networks[0].Nodes[0]
	if !node.Premium {
		t.Fatalf("expected node to be marked premium")
	}
	if node.PremiumStatus != "locked" {
		t.Fatalf("expected locked status, got %q", node.PremiumStatus)
	}
	if node.HTTPS != "https://api-hyperliquid-mainnet-orderbook.n.dwellir.com/<key>/ws" {
		t.Fatalf("expected locked endpoint URL to remain visible")
	}
	if node.WSS != "wss://api-hyperliquid-mainnet-orderbook.n.dwellir.com/<key>/ws" {
		t.Fatalf("expected locked endpoint URL to remain visible")
	}
}

func TestApplyPremiumEndpointLabelsTrialActiveKeepsEndpointAndTrialEnd(t *testing.T) {
	trialEnds := time.Now().UTC().Add(48 * time.Hour).Format(time.RFC3339)
	chains := []Chain{
		{
			Name: "Hyperliquid HyperCore Orderbook",
			Networks: []Network{
				{
					Name: "Mainnet",
					Nodes: []Node{
						{
							HTTPS:    "https://api-hyperliquid-mainnet-orderbook.n.dwellir.com/<key>/ws",
							NodeType: NodeType{Name: "Full"},
						},
					},
				},
			},
		},
	}
	account := &AccountInfo{
		PremiumEndpointState: PremiumEndpointState{
			{
				HostSlug:    "api-hyperliquid-mainnet-orderbook",
				Status:      "trial-active",
				TrialEndsAt: trialEnds,
			},
		},
	}

	out := ApplyPremiumEndpointLabels(chains, account)
	node := out[0].Networks[0].Nodes[0]
	if node.PremiumStatus != "trial-active" {
		t.Fatalf("expected trial-active status, got %q", node.PremiumStatus)
	}
	if node.TrialEndsAt != trialEnds {
		t.Fatalf("expected trial end %q, got %q", trialEnds, node.TrialEndsAt)
	}
	if node.HTTPS == "" {
		t.Fatalf("expected active trial endpoint URL to remain visible")
	}
}

// The Hyperliquid testnet SKUs were absent from premiumEndpointRules while the
// frontend already listed them, so the CLI rendered them as standard endpoints.
func TestApplyPremiumEndpointLabelsCoversEveryLockedEndpointRule(t *testing.T) {
	tests := []struct {
		name     string
		hostSlug string
		addOnUID string
	}{
		{name: "hyperliquid mainnet orderbook", hostSlug: "api-hyperliquid-mainnet-orderbook", addOnUID: "gWKew2Qp"},
		{name: "hyperliquid testnet orderbook", hostSlug: "api-hyperliquid-testnet-orderbook", addOnUID: "amRyMMWJ"},
		{name: "hyperliquid mainnet grpc", hostSlug: "api-hyperliquid-mainnet-grpc", addOnUID: "wQX7GZmK"},
		{name: "hyperliquid testnet grpc", hostSlug: "api-hyperliquid-testnet-grpc", addOnUID: "pWrOoamn"},
		{name: "asset hub kusama sidecar", hostSlug: "api-asset-hub-kusama-sidecar", addOnUID: "79OaqZmE"},
		{name: "asset hub polkadot sidecar", hostSlug: "api-asset-hub-polkadot-sidecar", addOnUID: "1Qp5KY9E"},
		{name: "assethub polkadot sidecar alias", hostSlug: "api-assethub-polkadot-sidecar", addOnUID: "1Qp5KY9E"},
		{name: "assethub kusama sidecar alias", hostSlug: "api-assethub-kusama-sidecar", addOnUID: "79OaqZmE"},
		{name: "kusama sidecar", hostSlug: "api-kusama-sidecar", addOnUID: "y9gpzRWM"},
		{name: "polkadot sidecar", hostSlug: "api-polkadot-sidecar", addOnUID: "E9L0oP9w"},
		{name: "centrifuge sidecar", hostSlug: "api-centrifuge-sidecar", addOnUID: "rm06Nk9X"},
		{name: "kilt sidecar", hostSlug: "api-kilt-sidecar", addOnUID: "z9MvOKW4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chains := []Chain{{
				Name: tt.hostSlug,
				Networks: []Network{{
					Name:  "Mainnet",
					Nodes: []Node{{HTTPS: "https://" + tt.hostSlug + ".n.dwellir.com/<key>"}},
				}},
			}}

			locked := ApplyPremiumEndpointLabels(chains, nil)[0].Networks[0].Nodes[0]
			if !locked.Premium {
				t.Fatalf("expected %s to be labelled premium", tt.hostSlug)
			}
			if locked.PremiumStatus != string(PremiumStatusLocked) {
				t.Fatalf("expected locked status without an add-on, got %q", locked.PremiumStatus)
			}

			account := &AccountInfo{CurrentSubscription: &CurrentSubscriptionWindow{
				SubscriptionAddOns: []OutsetaSubscriptionAddOn{{AddOnUID: tt.addOnUID}},
			}}
			active := ApplyPremiumEndpointLabels(chains, account)[0].Networks[0].Nodes[0]
			if active.PremiumStatus != string(PremiumStatusAddonActive) {
				t.Fatalf("expected add-on %s to unlock %s, got %q", tt.addOnUID, tt.hostSlug, active.PremiumStatus)
			}
		})
	}
}

func TestApplyPremiumEndpointLabelsLeavesUnlistedEndpointsStandard(t *testing.T) {
	chains := []Chain{{
		Name: "Ethereum",
		Networks: []Network{{
			Name:  "Mainnet",
			Nodes: []Node{{HTTPS: "https://api-ethereum-mainnet.n.dwellir.com/<key>"}},
		}},
	}}

	node := ApplyPremiumEndpointLabels(chains, nil)[0].Networks[0].Nodes[0]
	if node.Premium {
		t.Fatalf("expected a non-premium endpoint to stay standard")
	}
}
