package cli

import (
	"errors"
	"reflect"
	"testing"

	"github.com/dwellir-public/cli/internal/api"
	"github.com/dwellir-public/cli/internal/output"
)

func TestStatusCommandIsRegistered(t *testing.T) {
	for _, cmd := range rootCmd.Commands() {
		if cmd == statusCmd {
			return
		}
	}
	t.Fatal("status command is not registered on the root command")
}

func TestStatusSubcommandsAndFlags(t *testing.T) {
	want := map[string][]string{
		"components":  {"chain"},
		"history":     {"chain", "network", "endpoint", "since", "status", "limit"},
		"uptime":      {"chain", "network", "endpoint", "range"},
		"feed":        {"chain", "network", "endpoint", "format", "slack", "no-validate"},
		"subscribe":   {"chain", "network", "endpoint", "url", "min-severity"},
		"unsubscribe": {},
	}
	found := map[string]bool{}
	for _, sub := range statusCmd.Commands() {
		flags, ok := want[sub.Name()]
		if !ok {
			continue
		}
		found[sub.Name()] = true
		for _, name := range flags {
			if sub.Flags().Lookup(name) == nil {
				t.Errorf("status %s is missing --%s", sub.Name(), name)
			}
		}
		if sub.InheritedFlags().Lookup("status-url") == nil {
			t.Errorf("status %s does not inherit --status-url", sub.Name())
		}
	}
	for name := range want {
		if !found[name] {
			t.Errorf("status subcommand %q is not registered", name)
		}
	}
	for _, name := range []string{"chain", "network", "endpoint", "all"} {
		if statusCmd.Flags().Lookup(name) == nil {
			t.Errorf("status is missing --%s", name)
		}
	}
	if statusCmd.PersistentFlags().Lookup("status-url") == nil {
		t.Error("status is missing the persistent --status-url flag")
	}
}

func TestResolveStatusBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		flag    string
		env     string
		want    string
		wantErr bool
	}{
		{name: "default", want: api.DefaultStatusBaseURL},
		{name: "env", env: "http://localhost:3002/", want: "http://localhost:3002"},
		{name: "flag wins over env", flag: "https://staging.example.com", env: "http://localhost:3002", want: "https://staging.example.com"},
		{name: "no scheme", flag: "status.dwellir.com", wantErr: true},
		{name: "wrong scheme", flag: "ftp://status.dwellir.com", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(statusURLEnvVar, tt.env)
			previous := statusBaseURLFlag
			statusBaseURLFlag = tt.flag
			t.Cleanup(func() { statusBaseURLFlag = previous })

			got, err := resolveStatusBaseURL()
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("resolveStatusBaseURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClassifyStatusAPIError(t *testing.T) {
	tests := []struct {
		status   int
		message  string
		wantCode string
		wantExit int
	}{
		{status: 400, message: "Unknown chain: x", wantCode: "validation_error", wantExit: 4},
		{status: 404, wantCode: "not_found", wantExit: 3},
		{status: 409, message: "exists", wantCode: "conflict", wantExit: 1},
		{status: 429, message: "slow down", wantCode: "rate_limited", wantExit: 1},
		{status: 500, wantCode: "status_unavailable", wantExit: 1},
		{status: 503, message: "Subscriptions are unavailable", wantCode: "status_unavailable", wantExit: 1},
		{status: 418, wantCode: "api_error", wantExit: 1},
	}
	for _, tt := range tests {
		t.Run(tt.wantCode, func(t *testing.T) {
			code, message, _ := classifyStatusAPIError(&api.StatusAPIError{StatusCode: tt.status, Message: tt.message})
			if code != tt.wantCode {
				t.Fatalf("code = %q, want %q", code, tt.wantCode)
			}
			if message == "" {
				t.Fatal("expected a message")
			}
			if got := exitCodeFor(code); got != tt.wantExit {
				t.Fatalf("exitCodeFor(%q) = %d, want %d", code, got, tt.wantExit)
			}
		})
	}
}

func TestExitCode(t *testing.T) {
	rendered := &output.RenderedError{Code: "validation_error", Message: "bad"}
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "nil", err: nil, want: 0},
		{name: "plain error exits 1", err: errors.New("boom"), want: 1},
		{name: "rendered error without a code exits 1", err: rendered, want: 1},
		{name: "validation", err: withExitCode(rendered, "validation_error"), want: 4},
		{name: "not found", err: withExitCode(rendered, "not_found"), want: 3},
		{name: "auth", err: withExitCode(rendered, "not_authenticated"), want: 2},
		{name: "other", err: withExitCode(rendered, "status_unavailable"), want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExitCode(tt.err); got != tt.want {
				t.Fatalf("ExitCode() = %d, want %d", got, tt.want)
			}
		})
	}
	// Execute relies on errors.As to see the rendered error through the wrapper.
	if !output.IsRenderedError(withExitCode(rendered, "not_found")) {
		t.Fatal("withExitCode must keep the rendered error visible to errors.As")
	}
}

func statusComponentsFixture() *api.StatusComponents {
	return &api.StatusComponents{Chains: []api.StatusComponentChain{
		{Slug: "base", Networks: []api.StatusComponentNetwork{
			{Ref: "base/mainnet", Endpoints: []api.StatusEndpointRef{{FQDN: "api-base-mainnet.n.dwellir.com"}}},
		}},
		{Slug: "ethereum", Networks: []api.StatusComponentNetwork{
			{Ref: "ethereum/mainnet", Endpoints: []api.StatusEndpointRef{{FQDN: "api-ethereum-mainnet.n.dwellir.com"}}},
			{Ref: "ethereum/sepolia", Endpoints: []api.StatusEndpointRef{{FQDN: "api-ethereum-sepolia.n.dwellir.com"}}},
		}},
	}}
}

func TestFindUnknownFilterValues(t *testing.T) {
	tests := []struct {
		name   string
		filter api.StatusFilter
		want   []unknownFilterValue
	}{
		{name: "empty filter", filter: api.StatusFilter{}, want: []unknownFilterValue{}},
		{
			name:   "known values",
			filter: api.NewStatusFilter([]string{"base"}, []string{"ethereum/mainnet"}, []string{"https://api-base-mainnet.n.dwellir.com/key"}),
			want:   []unknownFilterValue{},
		},
		{
			name:   "typo in chain",
			filter: api.NewStatusFilter([]string{"etherum"}, nil, nil),
			want:   []unknownFilterValue{{Kind: "chain", Value: "etherum", Suggestions: []string{"ethereum"}}},
		},
		{
			name:   "network key without chain",
			filter: api.NewStatusFilter(nil, []string{"ethereum"}, nil),
			want: []unknownFilterValue{{
				Kind: "network", Value: "ethereum", Suggestions: []string{"ethereum/mainnet", "ethereum/sepolia"},
			}},
		},
		{
			name:   "nothing close",
			filter: api.NewStatusFilter(nil, nil, []string{"rpc.example.org"}),
			want:   []unknownFilterValue{{Kind: "endpoint", Value: "rpc.example.org", Suggestions: []string{}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findUnknownFilterValues(tt.filter, statusComponentsFixture())
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("findUnknownFilterValues() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestFilterComponentChains(t *testing.T) {
	components := statusComponentsFixture()
	if got := filterComponentChains(components, nil); len(got.Chains) != 2 {
		t.Fatalf("no filter must keep every chain, got %d", len(got.Chains))
	}
	got := filterComponentChains(components, []string{"ethereum"})
	if len(got.Chains) != 1 || got.Chains[0].Slug != "ethereum" {
		t.Fatalf("filterComponentChains() = %#v", got.Chains)
	}
}

func TestLevenshtein(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"base", "base", 0},
		{"etherum", "ethereum", 1},
		{"kitten", "sitting", 3},
	}
	for _, tt := range tests {
		if got := levenshtein(tt.a, tt.b); got != tt.want {
			t.Errorf("levenshtein(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}
