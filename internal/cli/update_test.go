package cli

import "testing"

func TestIsLatestVersion(t *testing.T) {
	tests := []struct {
		name     string
		current  string
		latest   string
		expected bool
		wantErr  bool
	}{
		{
			name:     "same version is up to date",
			current:  "0.1.2",
			latest:   "0.1.2",
			expected: true,
		},
		{
			name:     "newer current version is up to date",
			current:  "0.1.3",
			latest:   "0.1.2",
			expected: true,
		},
		{
			name:     "older current version is not up to date",
			current:  "0.1.1",
			latest:   "0.1.2",
			expected: false,
		},
		{
			name:     "dev build is treated as not up to date",
			current:  "dev",
			latest:   "0.1.2",
			expected: false,
		},
		{
			name:    "invalid latest version returns error",
			current: "0.1.2",
			latest:  "latest",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := isLatestVersion(test.current, test.latest)
			if test.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != test.expected {
				t.Fatalf("expected %v, got %v", test.expected, got)
			}
		})
	}
}
