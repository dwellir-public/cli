package api

import "strings"

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
	ID       int      `json:"id"`
	HTTPS    string   `json:"https"`
	WSS      string   `json:"wss"`
	NodeType NodeType `json:"node_type"`
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
			if !matchesNetworkFilter(net.Name, network) {
				continue
			}

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
				matchedNodes = append(matchedNodes, node)
			}

			if len(matchedNodes) > 0 {
				net.Nodes = matchedNodes
				matchedNetworks = append(matchedNetworks, net)
			}
		}

		if len(matchedNetworks) > 0 {
			chain.Networks = matchedNetworks
			filtered = append(filtered, chain)
		}
	}

	return filtered, nil
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
