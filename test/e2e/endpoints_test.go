//go:build e2e

package e2e

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEndpointsNetworkFilter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v3/chains" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`[
  {
    "id": 1,
    "name": "Ethereum",
    "image_url": "eth.png",
    "ecosystem": "evm",
    "networks": [
      {
        "id": 1,
        "name": "Mainnet",
        "nodes": [
          {"id": 1, "https": "https://mainnet.example", "wss": "wss://mainnet.example", "node_type": {"name": "full"}}
        ]
      },
      {
        "id": 2,
        "name": "Sepolia Testnet",
        "nodes": [
          {"id": 2, "https": "https://sepolia.example", "wss": "wss://sepolia.example", "node_type": {"name": "archive"}}
        ]
      }
    ]
  }
]`))
	}))
	defer server.Close()

	res := runCLIWithEnv(t, map[string]string{
		"DWELLIR_TOKEN":   "test-token",
		"DWELLIR_API_URL": server.URL,
	}, "endpoints", "list", "--network", "mainnet", "--json")

	if res.exitCode != 0 {
		t.Fatalf("expected success exit code, got %d\nstderr: %s\nstdout: %s", res.exitCode, res.stderr, res.stdout)
	}

	parsed := parseJSON(t, res.stdout)
	data, _ := parsed["data"].([]interface{})
	if len(data) != 1 {
		t.Fatalf("expected one chain, got %d", len(data))
	}

	chain := data[0].(map[string]interface{})
	networks, _ := chain["networks"].([]interface{})
	if len(networks) != 1 {
		t.Fatalf("expected one filtered network, got %d", len(networks))
	}
	network := networks[0].(map[string]interface{})
	if network["name"] != "Mainnet" {
		t.Fatalf("expected Mainnet, got %v", network["name"])
	}
}
