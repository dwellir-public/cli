package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type testChain struct {
	ID       int           `json:"id"`
	Name     string        `json:"name"`
	ImageURL string        `json:"image_url"`
	Networks []testNetwork `json:"networks"`
}

type testNetwork struct {
	ID    int        `json:"id"`
	Name  string     `json:"name"`
	Nodes []testNode `json:"nodes"`
}

type testNode struct {
	ID       int          `json:"id"`
	HTTPS    string       `json:"https"`
	WSS      string       `json:"wss"`
	NodeType testNodeType `json:"node_type"`
}

type testNodeType struct {
	Name string `json:"name"`
}

func TestListEndpoints(t *testing.T) {
	chains := []testChain{
		{
			ID: 1, Name: "Ethereum", ImageURL: "eth.png",
			Networks: []testNetwork{
				{
					ID: 1, Name: "Mainnet",
					Nodes: []testNode{
						{ID: 1, HTTPS: "https://eth.dwellir.com", WSS: "wss://eth.dwellir.com", NodeType: testNodeType{Name: "full"}},
					},
				},
			},
		},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(chains); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	ep := NewEndpointsAPI(client)
	result, err := ep.List()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 chain, got %d", len(result))
	}
	if result[0].Name != "Ethereum" {
		t.Errorf("expected Ethereum, got %s", result[0].Name)
	}
}

func TestSearchEndpointsByNetworkFilter(t *testing.T) {
	chains := []testChain{
		{
			ID: 1, Name: "Ethereum", ImageURL: "eth.png",
			Networks: []testNetwork{
				{
					ID: 1, Name: "Mainnet",
					Nodes: []testNode{
						{ID: 1, HTTPS: "https://eth-mainnet.dwellir.com", WSS: "wss://eth-mainnet.dwellir.com", NodeType: testNodeType{Name: "full"}},
					},
				},
				{
					ID: 2, Name: "Sepolia Testnet",
					Nodes: []testNode{
						{ID: 2, HTTPS: "https://eth-sepolia.dwellir.com", WSS: "wss://eth-sepolia.dwellir.com", NodeType: testNodeType{Name: "archive"}},
					},
				},
			},
		},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(chains); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	ep := NewEndpointsAPI(client)

	mainnetOnly, err := ep.Search("", "", "", "", "mainnet")
	if err != nil {
		t.Fatalf("unexpected error filtering mainnet: %v", err)
	}
	if len(mainnetOnly) != 1 || len(mainnetOnly[0].Networks) != 1 || mainnetOnly[0].Networks[0].Name != "Mainnet" {
		t.Fatalf("expected only mainnet network, got %+v", mainnetOnly)
	}

	testnetOnly, err := ep.Search("", "", "", "", "testnet")
	if err != nil {
		t.Fatalf("unexpected error filtering testnet: %v", err)
	}
	if len(testnetOnly) != 1 || len(testnetOnly[0].Networks) != 1 || testnetOnly[0].Networks[0].Name != "Sepolia Testnet" {
		t.Fatalf("expected only testnet network, got %+v", testnetOnly)
	}

	sepoliaByName, err := ep.Search("", "", "", "", "sepolia")
	if err != nil {
		t.Fatalf("unexpected error filtering by network name: %v", err)
	}
	if len(sepoliaByName) != 1 || len(sepoliaByName[0].Networks) != 1 || sepoliaByName[0].Networks[0].Name != "Sepolia Testnet" {
		t.Fatalf("expected sepolia network by name filter, got %+v", sepoliaByName)
	}
}
