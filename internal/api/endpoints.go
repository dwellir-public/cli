package api

import (
	"strings"
	"unicode"
)

type Chain struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	ImageURL  string    `json:"image_url"`
	Ecosystem string    `json:"ecosystem,omitempty"`
	Networks  []Network `json:"networks"`
}

type Network struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Nodes []Node `json:"nodes"`
}

type Node struct {
	ID            int      `json:"id"`
	HTTPS         string   `json:"https"`
	WSS           string   `json:"wss"`
	NodeType      NodeType `json:"node_type"`
	Premium       bool     `json:"premium,omitempty"`
	PremiumStatus string   `json:"premiumStatus,omitempty"`
	TrialEndsAt   string   `json:"trialEndsAt,omitempty"`
}

type NodeType struct {
	Name string `json:"name"`
}

type EndpointsAPI struct {
	client *Client
}

func NewEndpointsAPI(client *Client) *EndpointsAPI {
	return &EndpointsAPI{client: client}
}

func (e *EndpointsAPI) List() ([]Chain, error) {
	var chains []Chain
	err := e.client.Get("/v3/chains", nil, &chains)
	return chains, err
}

// Search filters chains by query string and optional endpoint filters.
func (e *EndpointsAPI) Search(query string, ecosystem string, nodeType string, protocol string, network string) ([]Chain, error) {
	chains, err := e.List()
	if err != nil {
		return nil, err
	}

	query = strings.ToLower(query)
	var filtered []Chain

	for _, chain := range chains {
		if ecosystem != "" && !strings.EqualFold(chain.Ecosystem, ecosystem) {
			continue
		}
		chainMatch := query == "" || strings.Contains(strings.ToLower(chain.Name), query)

		var matchedNetworks []Network
		for _, net := range chain.Networks {
			netMatch := chainMatch || strings.Contains(strings.ToLower(net.Name), query)
			if !netMatch {
				continue
			}
			if filteredNetwork, ok := filterNetwork(net, nodeType, protocol, network); ok {
				matchedNetworks = append(matchedNetworks, filteredNetwork)
			}
		}

		if len(matchedNetworks) > 0 {
			chain.Networks = matchedNetworks
			filtered = append(filtered, chain)
		}
	}

	return filtered, nil
}

// Get finds one specific chain by exact name/slug match and applies endpoint filters.
func (e *EndpointsAPI) Get(chainLookup string, ecosystem string, nodeType string, protocol string, network string) ([]Chain, error) {
	chains, err := e.List()
	if err != nil {
		return nil, err
	}

	lookup := strings.TrimSpace(chainLookup)
	if lookup == "" {
		return nil, nil
	}

	for _, chain := range chains {
		if ecosystem != "" && !strings.EqualFold(chain.Ecosystem, ecosystem) {
			continue
		}
		if !matchesChainLookup(chain.Name, lookup) {
			continue
		}

		var matchedNetworks []Network
		for _, net := range chain.Networks {
			if filteredNetwork, ok := filterNetwork(net, nodeType, protocol, network); ok {
				matchedNetworks = append(matchedNetworks, filteredNetwork)
			}
		}

		if len(matchedNetworks) == 0 {
			return nil, nil
		}
		chain.Networks = matchedNetworks
		return []Chain{chain}, nil
	}

	return nil, nil
}

func matchesNetworkFilter(networkName, filter string) bool {
	filter = strings.ToLower(strings.TrimSpace(filter))
	if filter == "" {
		return true
	}

	name := strings.ToLower(strings.TrimSpace(networkName))
	switch filter {
	case "mainnet":
		return strings.Contains(name, "mainnet")
	case "testnet":
		return !strings.Contains(name, "mainnet")
	default:
		return strings.Contains(name, filter)
	}
}

func filterNetwork(net Network, nodeType string, protocol string, network string) (Network, bool) {
	if !matchesNetworkFilter(net.Name, network) {
		return Network{}, false
	}

	nodeType = strings.TrimSpace(nodeType)
	protocol = strings.ToLower(strings.TrimSpace(protocol))

	var matchedNodes []Node
	for _, node := range net.Nodes {
		if nodeType != "" && !strings.EqualFold(node.NodeType.Name, nodeType) {
			continue
		}
		if protocol == "https" && node.HTTPS == "" {
			continue
		}
		if protocol == "wss" && node.WSS == "" {
			continue
		}
		filteredNode := node
		switch protocol {
		case "https":
			filteredNode.WSS = ""
		case "wss":
			filteredNode.HTTPS = ""
		}
		matchedNodes = append(matchedNodes, filteredNode)
	}

	if len(matchedNodes) == 0 {
		return Network{}, false
	}

	net.Nodes = matchedNodes
	return net, true
}

func matchesChainLookup(chainName string, lookup string) bool {
	name := strings.TrimSpace(chainName)
	target := strings.TrimSpace(lookup)
	if strings.EqualFold(name, target) {
		return true
	}
	return normalizeChainLookup(name) == normalizeChainLookup(target)
}

func normalizeChainLookup(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}

	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
