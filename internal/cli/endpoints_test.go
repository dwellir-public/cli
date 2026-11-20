package cli

import (
	"strings"
	"testing"

	"github.com/dwellir-public/cli/internal/api"
	"github.com/dwellir-public/cli/internal/output"
)

func TestEndpointNoticesExplainHowToSendTheKey(t *testing.T) {
	chains := func(node api.Node) []api.Chain {
		return []api.Chain{{Networks: []api.Network{{Nodes: []api.Node{node}}}}}
	}
	const https, grpc = "https://api-x.n.dwellir.com/<key>", "api-x.n.dwellir.com:443"

	tests := []struct {
		name            string
		node            api.Node
		wantPlaceholder bool
		wantHeader      bool
	}{
		{"https only", api.Node{HTTPS: https}, true, false},
		{"grpc only", api.Node{GRPC: grpc}, false, true},
		{"both", api.Node{HTTPS: https, GRPC: grpc}, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notice, ok := withEndpointNotices(chains(tt.node)).(output.Notice)
			if !ok {
				t.Fatal("expected a notice")
			}
			if strings.Contains(notice.Text, "<key> placeholder") != tt.wantPlaceholder {
				t.Errorf("placeholder notice: got %q", notice.Text)
			}
			if strings.Contains(notice.Text, "x-api-key metadata header") != tt.wantHeader {
				t.Errorf("x-api-key notice: got %q", notice.Text)
			}
		})
	}

	if _, ok := withEndpointNotices(chains(api.Node{HTTPS: "[locked premium endpoint]"})).(output.Notice); ok {
		t.Error("expected no notice for an endpoint with no key placeholder and no gRPC target")
	}
}
