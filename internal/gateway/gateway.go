package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

// Gateway selects the first healthy backend for read-only requests.
type Gateway struct {
	primary   *url.URL
	secondary *url.URL
	client    *http.Client
}

type Status struct {
	Primary   bool   `json:"primary_ready"`
	Secondary bool   `json:"standby_ready"`
	Active    string `json:"active"`
}

func New(primary, standby string) (*Gateway, error) {
	p, err := backendURL(primary)
	if err != nil {
		return nil, err
	}
	s, err := backendURL(standby)
	if err != nil {
		return nil, err
	}
	return &Gateway{
		primary: p, secondary: s,
		client: &http.Client{
			Timeout: time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

func backendURL(value string) (*url.URL, error) {
	u, err := url.Parse(value)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" ||
		(u.Scheme != "http" && u.Scheme != "https") || (u.Path != "" && u.Path != "/") {
		return nil, errors.New("expected an HTTP(S) origin without credentials, path or query")
	}
	return u, nil
}

func (g *Gateway) ready(ctx context.Context, backend *url.URL) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, backend.ResolveReference(&url.URL{Path: "/healthz"}).String(), nil)
	if err != nil {
		return false
	}
	response, err := g.client.Do(req)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	return response.StatusCode >= 200 && response.StatusCode < 300
}

func (g *Gateway) Status(ctx context.Context) Status {
	status := Status{Primary: g.ready(ctx, g.primary), Secondary: g.ready(ctx, g.secondary), Active: "unavailable"}
	if status.Primary {
		status.Active = "primary"
	} else if status.Secondary {
		status.Active = "standby"
	}
	return status
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "read-only gateway", http.StatusMethodNotAllowed)
		return
	}
	status := g.Status(request.Context())
	if request.URL.Path == "/_control/status" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(status)
		return
	}
	target := g.primary
	switch status.Active {
	case "standby":
		target = g.secondary
	case "unavailable":
		http.Error(w, "no ready backend", http.StatusServiceUnavailable)
		return
	}
	proxy := httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(target)
			r.Out.Header.Del("Authorization")
			r.Out.Header.Del("Cookie")
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
			http.Error(w, "backend request failed", http.StatusBadGateway)
		},
	}
	w.Header().Set("X-Starteam-Backend", status.Active)
	proxy.ServeHTTP(w, request)
}
