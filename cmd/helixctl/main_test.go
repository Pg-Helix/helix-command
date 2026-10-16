package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func okDial(context.Context, string) error  { return nil }
func badDial(context.Context, string) error { return errors.New("refused") }

func exec(t *testing.T, dial func(context.Context, string) error, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(args, &out, &errb, dial)
	return code, out.String(), errb.String()
}

func writeCfg(t *testing.T, body string) string {
	p := filepath.Join(t.TempDir(), "c.json")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestVersion(t *testing.T) {
	code, out, _ := exec(t, okDial, "version")
	if code != 0 || out != "helixctl dev\n" {
		t.Fatalf("got %d %q", code, out)
	}
	_, out, _ = exec(t, okDial, "version", "--json")
	if out != "{\n  \"version\": \"dev\"\n}\n" {
		t.Fatalf("json: %q", out)
	}
}

func TestUsageErrors(t *testing.T) {
	for _, args := range [][]string{{}, {"nope"}, {"version", "--bogus"}, {"doctor", "--config", "/no/such/file"}} {
		if code, _, _ := exec(t, okDial, args...); code != 2 {
			t.Errorf("%v: exit %d, want 2", args, code)
		}
	}
	p := writeCfg(t, "{not json")
	if code, _, e := exec(t, okDial, "doctor", "--config", p); code != 2 || !strings.Contains(e, "invalid config") {
		t.Errorf("bad json: %d %q", code, e)
	}
}

func TestDoctorNoConfigSkips(t *testing.T) {
	code, out, _ := exec(t, badDial, "doctor")
	if code != 0 || !strings.Contains(out, "skip") {
		t.Fatalf("got %d %q", code, out)
	}
}

func TestDoctorReachability(t *testing.T) {
	p := writeCfg(t, `{"endpoint":"https://helix.example","database":"db:5432"}`)
	if code, out, _ := exec(t, okDial, "doctor", "--config", p); code != 0 || strings.Contains(out, "fail") {
		t.Fatalf("ok case: %d %q", code, out)
	}
	code, out, _ := exec(t, badDial, "doctor", "--config", p, "--json")
	if code != 1 || !strings.Contains(out, `"status": "fail"`) || !strings.Contains(out, "helix.example:443") {
		t.Fatalf("fail case: %d %q", code, out)
	}
}

func TestDoctorMalformedEndpointAndEmptyConfig(t *testing.T) {
	p := writeCfg(t, `{"endpoint":"ftp://x"}`)
	if code, out, _ := exec(t, okDial, "doctor", "--config", p); code != 1 || !strings.Contains(out, "malformed") {
		t.Fatalf("%d %q", code, out)
	}
	p = writeCfg(t, `{}`)
	if code, _, _ := exec(t, okDial, "doctor", "--config", p); code != 1 {
		t.Fatalf("empty config exit %d", code)
	}
}
