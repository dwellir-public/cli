//go:build e2e

package e2e

import (
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestEndpointsHelpShowsFilters(t *testing.T) {
	res := runCLI(t, "endpoints", "--help")
	if res.exitCode != 0 {
		t.Fatalf("expected success exit code, got %d\nstderr: %s\nstdout: %s", res.exitCode, res.stderr, res.stdout)
	}

	if !strings.Contains(res.stdout, "--network") ||
		!strings.Contains(res.stdout, "--ecosystem") ||
		!strings.Contains(res.stdout, "--node-type") ||
		!strings.Contains(res.stdout, "--protocol") {
		t.Fatalf("expected endpoints help to mention filter flags, got:\n%s", res.stdout)
	}
}

func TestEndpointsShowGRPCTarget(t *testing.T) {
	const target = "api-hyperliquid-mainnet-grpc.n.dwellir.com:443"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v3/chains" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`[{"id": 1, "name": "Hyperliquid", "ecosystem": "hyperliquid", "networks": [{"id": 1, "name": "Mainnet", "nodes": [
  {"id": 1, "https": "https://api-hyperliquid-mainnet.n.dwellir.com/<key>", "wss": null, "node_type": {"name": "Full"}},
  {"id": 2, "https": null, "wss": null, "grpc": "` + target + `", "node_type": {"name": "Full"}}
]}]}]`))
	}))
	defer server.Close()
	env := map[string]string{"DWELLIR_TOKEN": "test-token", "DWELLIR_API_URL": server.URL}

	res := runCLIWithEnv(t, env, "endpoints", "list", "--protocol", "grpc", "--json")
	if res.exitCode != 0 {
		t.Fatalf("expected success exit code, got %d\nstderr: %s\nstdout: %s", res.exitCode, res.stderr, res.stdout)
	}
	parsed := parseJSON(t, res.stdout)
	nodes := parsed["data"].([]interface{})[0].(map[string]interface{})["networks"].([]interface{})[0].(map[string]interface{})["nodes"].([]interface{})
	if len(nodes) != 1 || nodes[0].(map[string]interface{})["grpc"] != target {
		t.Fatalf("expected only the gRPC node with its target, got %v", nodes)
	}
	notice, _ := parsed["meta"].(map[string]interface{})["notice"].(string)
	if !strings.Contains(notice, "x-api-key metadata header") {
		t.Fatalf("expected the x-api-key notice, got %q", notice)
	}

	res = runCLIWithEnv(t, env, "endpoints", "list", "--human")
	if res.exitCode != 0 || !strings.Contains(res.stdout, target) || !strings.Contains(res.stdout, "x-api-key metadata header") {
		t.Fatalf("expected the gRPC target and the x-api-key note in human output, got %d\n%s", res.exitCode, res.stdout)
	}
}
