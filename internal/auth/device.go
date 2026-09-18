package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dwellir-public/cli/internal/config"
)

type deviceGrant struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri_complete"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// DeviceLogin works when the user's browser cannot reach a localhost callback.
func DeviceLogin(ctx context.Context, configDir, profileName, apiURL string) (*config.Profile, error) {
	client := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	base := strings.TrimRight(apiURL, "/") + "/v4/auth/device"
	var grant deviceGrant
	if err := deviceRequest(ctx, client, base+"/code", map[string]string{"scope": "read-write"}, &grant); err != nil {
		return nil, err
	}
	if grant.DeviceCode == "" || grant.VerificationURI == "" || grant.ExpiresIn <= 0 || grant.ExpiresIn > 900 || grant.Interval > 60 {
		return nil, errors.New("invalid device authorization response")
	}
	fmt.Fprintf(config.Stderr(), "Approve Dwellir access in your browser:\n%s\nCode: %s\n", grant.VerificationURI, grant.UserCode)
	ctx, cancel := context.WithTimeout(ctx, time.Duration(grant.ExpiresIn)*time.Second)
	defer cancel()
	interval := time.Duration(max(grant.Interval, 1)) * time.Second
	token, err := pollDevice(ctx, client, base+"/token", grant.DeviceCode, interval)
	if err != nil {
		return nil, err
	}
	if profileName == "" {
		profileName = "default"
	}
	p := &config.Profile{Name: profileName, Token: token}
	if err := config.SaveProfile(configDir, p); err != nil {
		return nil, errors.New("could not save device login")
	}
	return p, nil
}

func pollDevice(ctx context.Context, client *http.Client, endpoint, code string, interval time.Duration) (string, error) {
	for {
		var reply struct {
			Token string `json:"access_token"`
			Error string `json:"error"`
		}
		if err := deviceRequest(ctx, client, endpoint, map[string]string{"device_code": code}, &reply); err != nil {
			return "", err
		}
		if reply.Token != "" {
			return reply.Token, nil
		}
		switch reply.Error {
		case "authorization_pending":
		case "slow_down":
			interval += 5 * time.Second
		case "access_denied":
			return "", errors.New("device authorization was denied")
		case "expired_token":
			return "", errors.New("device authorization expired; run login again")
		default:
			return "", errors.New("device authorization failed")
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return "", errors.New("device authorization cancelled or expired")
		case <-timer.C:
		}
	}
}

func deviceRequest(ctx context.Context, client *http.Client, endpoint string, payload any, result any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return errors.New("invalid device request")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return errors.New("invalid device authorization URL")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return errors.New("could not reach device authorization service")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 400 {
		return fmt.Errorf("device authorization returned HTTP %d", resp.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64*1024)).Decode(result); err != nil {
		return errors.New("invalid device authorization response")
	}
	return nil
}
