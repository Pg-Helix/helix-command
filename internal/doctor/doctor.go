// Package doctor runs local readiness checks for helixctl.
package doctor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"time"
)

// Config is the M1 config file (JSON; format is an open question in RFC-0002).
type Config struct {
	Endpoint string `json:"endpoint"` // helix-command base URL
	Database string `json:"database"` // host:port of PostgreSQL
}

type Check struct {
	Name   string `json:"name"`
	Status string `json:"status"` // ok | fail | skip
	Detail string `json:"detail,omitempty"`
}

// ErrConfig marks an unusable config file (CLI exit code 2).
var ErrConfig = errors.New("invalid config")

func Load(path string) (Config, error) {
	var c Config
	b, err := os.ReadFile(path)
	if err != nil {
		return c, fmt.Errorf("%w: %v", ErrConfig, err)
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return c, fmt.Errorf("%w: %s: %v", ErrConfig, path, err)
	}
	return c, nil
}

// Dialer reports whether addr accepts a TCP connection.
type Dialer func(ctx context.Context, addr string) error

func TCPDialer(timeout time.Duration) Dialer {
	return func(ctx context.Context, addr string) error {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		c, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
		if err != nil {
			return err
		}
		return c.Close()
	}
}

// Run executes checks; it never blocks longer than the dialer's timeout per target.
func Run(ctx context.Context, cfg *Config, dial Dialer) []Check {
	if cfg == nil {
		return []Check{{Name: "config", Status: "skip", Detail: "no --config given; network checks skipped"}}
	}
	out := []Check{{Name: "config", Status: "ok"}}
	if cfg.Endpoint == "" && cfg.Database == "" {
		return append(out, Check{Name: "config", Status: "fail", Detail: "neither endpoint nor database is set"})
	}
	if cfg.Endpoint != "" {
		out = append(out, reach(ctx, "endpoint", endpointAddr(cfg.Endpoint), dial))
	}
	if cfg.Database != "" {
		out = append(out, reach(ctx, "database", cfg.Database, dial))
	}
	return out
}

func endpointAddr(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return ""
	}
	port := u.Port()
	if port == "" {
		port = map[string]string{"http": "80", "https": "443"}[u.Scheme]
	}
	return net.JoinHostPort(u.Hostname(), port)
}

func reach(ctx context.Context, name, addr string, dial Dialer) Check {
	if addr == "" {
		return Check{Name: name, Status: "fail", Detail: "malformed value (want http(s)://host[:port])"}
	}
	if err := dial(ctx, addr); err != nil {
		return Check{Name: name, Status: "fail", Detail: fmt.Sprintf("%s unreachable: %v", addr, err)}
	}
	return Check{Name: name, Status: "ok", Detail: addr}
}

// Failed reports whether any check failed.
func Failed(cs []Check) bool {
	for _, c := range cs {
		if c.Status == "fail" {
			return true
		}
	}
	return false
}
