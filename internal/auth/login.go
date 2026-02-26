package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/dwellir-public/cli/internal/config"
)

const loginTimeout = 5 * time.Minute

type CallbackPayload struct {
	Token string `json:"token"`
	Org   string `json:"org"`
	User  string `json:"user"`
}

// Login starts the browser-based auth flow.
func Login(configDir string, profileName string, dashboardURL string) (*config.Profile, error) {
	if profileName == "" {
		profileName = "default"
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("starting local server: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	resultCh := make(chan *CallbackPayload, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload CallbackPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid payload", http.StatusBadRequest)
			errCh <- fmt.Errorf("invalid callback payload: %w", err)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", dashboardURL)
		w.Header().Set("Access-Control-Allow-Methods", "POST")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok"}`)
		resultCh <- &payload
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Origin", dashboardURL)
			w.Header().Set("Access-Control-Allow-Methods", "POST")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.WriteHeader(http.StatusNoContent)
		}
	})

	server := &http.Server{Handler: mux}
	go func() {
		if err := server.Serve(listener); err != http.ErrServerClosed {
			errCh <- err
		}
	}()
	defer func() {
		_ = server.Shutdown(context.Background())
	}()

	authURL := buildCLIAuthURL(dashboardURL, port, machineHostname())
	fmt.Fprintf(config.Stderr(), "Opening browser for authentication...\n")
	fmt.Fprintf(config.Stderr(), "If the browser doesn't open, visit:\n  %s\n\n", authURL)
	openBrowser(authURL)

	select {
	case payload := <-resultCh:
		p := &config.Profile{
			Name:  profileName,
			Token: payload.Token,
			Org:   payload.Org,
			User:  payload.User,
		}
		if err := config.SaveProfile(configDir, p); err != nil {
			return nil, fmt.Errorf("saving profile: %w", err)
		}
		return p, nil

	case err := <-errCh:
		return nil, err

	case <-time.After(loginTimeout):
		return nil, fmt.Errorf("authentication timed out after %s.\n\nFor headless/CI environments, create a token manually at:\n  %s/agents", loginTimeout, dashboardURL)
	}
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

func machineHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(hostname)
}

func buildCLIAuthURL(dashboardURL string, port int, device string) string {
	baseURL := strings.TrimRight(dashboardURL, "/")
	u, err := url.Parse(baseURL + "/cli-auth")
	if err != nil {
		fallback := fmt.Sprintf("%s/cli-auth?port=%d", baseURL, port)
		if device == "" {
			return fallback
		}
		return fallback + "&device=" + url.QueryEscape(device)
	}

	q := u.Query()
	q.Set("port", strconv.Itoa(port))
	if device != "" {
		q.Set("device", device)
	}
	u.RawQuery = q.Encode()
	return u.String()
}
