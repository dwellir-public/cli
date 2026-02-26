package cli

import (
	"errors"
	"strings"
	"testing"
)

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

func TestDetectManagedInstallHintWith(t *testing.T) {
	tests := []struct {
		name        string
		cmdPath     string
		lookPath    func(string) (string, error)
		runCmd      func(string, ...string) (string, error)
		contains    string
		notContains string
	}{
		{
			name:    "detects pacman/aur package",
			cmdPath: "/usr/bin/dwellir",
			lookPath: func(bin string) (string, error) {
				switch bin {
				case "pacman":
					return "/usr/bin/pacman", nil
				default:
					return "", errors.New("not found")
				}
			},
			runCmd: func(name string, args ...string) (string, error) {
				if name == "pacman" && len(args) == 2 && args[0] == "-Qo" && args[1] == "/usr/bin/dwellir" {
					return "/usr/bin/dwellir is owned by dwellir-cli-bin 0.1.4-1\n", nil
				}
				return "", errors.New("unexpected command")
			},
			contains: "yay -Syu dwellir-cli-bin",
		},
		{
			name:    "detects homebrew install",
			cmdPath: "/opt/homebrew/bin/dwellir",
			lookPath: func(bin string) (string, error) {
				if bin == "brew" {
					return "/opt/homebrew/bin/brew", nil
				}
				return "", errors.New("not found")
			},
			runCmd: func(name string, args ...string) (string, error) {
				if name == "brew" && len(args) == 3 && args[0] == "list" && args[1] == "--formula" && args[2] == "dwellir" {
					return "dwellir\n", nil
				}
				return "", errors.New("unexpected command")
			},
			contains: "brew upgrade dwellir",
		},
		{
			name:    "returns empty when no manager detected",
			cmdPath: "/tmp/dwellir",
			lookPath: func(string) (string, error) {
				return "", errors.New("not found")
			},
			runCmd: func(string, ...string) (string, error) {
				return "", errors.New("not found")
			},
			contains: "",
		},
		{
			name:    "falls back to lookPath when executable path empty",
			cmdPath: "",
			lookPath: func(bin string) (string, error) {
				switch bin {
				case "dwellir":
					return "/usr/bin/dwellir", nil
				case "pacman":
					return "/usr/bin/pacman", nil
				default:
					return "", errors.New("not found")
				}
			},
			runCmd: func(name string, args ...string) (string, error) {
				if name == "pacman" && len(args) == 2 && args[0] == "-Qo" && args[1] == "/usr/bin/dwellir" {
					return "/usr/bin/dwellir is owned by dwellir-cli-bin 0.1.4-1\n", nil
				}
				return "", errors.New("unexpected command")
			},
			contains: "pacman/AUR",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := detectManagedInstallHintWith(tc.cmdPath, tc.lookPath, tc.runCmd)
			if tc.contains == "" && got != "" {
				t.Fatalf("expected empty hint, got %q", got)
			}
			if tc.contains != "" && !strings.Contains(got, tc.contains) {
				t.Fatalf("expected hint to contain %q, got %q", tc.contains, got)
			}
			if tc.notContains != "" && strings.Contains(got, tc.notContains) {
				t.Fatalf("expected hint to not contain %q, got %q", tc.notContains, got)
			}
		})
	}
}

func TestParsePacmanOwnedPackage(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "parses package name",
			input:  "/usr/bin/dwellir is owned by dwellir-cli-bin 0.1.4-1\n",
			expect: "dwellir-cli-bin",
		},
		{
			name:   "returns empty for unexpected format",
			input:  "some other output",
			expect: "",
		},
		{
			name:   "returns empty for empty output",
			input:  "",
			expect: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parsePacmanOwnedPackage(tc.input)
			if got != tc.expect {
				t.Fatalf("expected %q, got %q", tc.expect, got)
			}
		})
	}
}
