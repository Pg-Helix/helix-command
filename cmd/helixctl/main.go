// Command helixctl is the PgHelix CLI. Syntax is a design target (see helix-atlas RFC-0002).
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/Pg-Helix/helix-command/internal/doctor"
)

// version is overridden at build time: -ldflags "-X main.version=..."
var version = "dev"

const (
	exitOK       = 0
	exitProblems = 1
	exitUsage    = 2
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, doctor.TCPDialer(3*time.Second)))
}

func run(args []string, stdout, stderr io.Writer, dial doctor.Dialer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: helixctl <version|doctor> [flags]")
		return exitUsage
	}
	switch args[0] {
	case "version":
		return cmdVersion(args[1:], stdout, stderr)
	case "doctor":
		return cmdDoctor(args[1:], stdout, stderr, dial)
	default:
		fmt.Fprintf(stderr, "helixctl: unknown command %q\n", args[0])
		return exitUsage
	}
}

func newFlags(name string, stderr io.Writer) (*flag.FlagSet, *bool) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	return fs, fs.Bool("json", false, "machine-readable output")
}

func cmdVersion(args []string, stdout, stderr io.Writer) int {
	fs, asJSON := newFlags("version", stderr)
	if fs.Parse(args) != nil {
		return exitUsage
	}
	if *asJSON {
		writeJSON(stdout, map[string]string{"version": version})
	} else {
		fmt.Fprintf(stdout, "helixctl %s\n", version)
	}
	return exitOK
}

func cmdDoctor(args []string, stdout, stderr io.Writer, dial doctor.Dialer) int {
	fs, asJSON := newFlags("doctor", stderr)
	path := fs.String("config", "", "config file (JSON: endpoint, database)")
	if fs.Parse(args) != nil {
		return exitUsage
	}
	var cfg *doctor.Config
	if *path != "" {
		c, err := doctor.Load(*path)
		if err != nil {
			fmt.Fprintf(stderr, "helixctl: %v\n", err)
			return exitUsage
		}
		cfg = &c
	}
	checks := doctor.Run(context.Background(), cfg, dial)
	if *asJSON {
		writeJSON(stdout, map[string]any{"checks": checks})
	} else {
		for _, c := range checks {
			fmt.Fprintf(stdout, "%-4s %-9s %s\n", c.Status, c.Name, c.Detail)
		}
	}
	if doctor.Failed(checks) {
		return exitProblems
	}
	return exitOK
}

func writeJSON(w io.Writer, v any) {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}
