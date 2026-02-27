package api

import (
	"encoding/json"
	"testing"
	"time"
)

func TestAccountInfoUnmarshalPremiumEndpointStateString(t *testing.T) {
	raw := `{
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
