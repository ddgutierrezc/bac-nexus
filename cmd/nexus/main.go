// Package main wires the v1 MCP stdio server entry point. The
// command composes the catalog-context service from phases 2, 3B.3,
// 5A, 5B, and 6, invokes the pre-acquire recovery gate during
// startup, and runs the MCP server over stdio. It must never
// expose a generic remote, path, shell, SQL, or SSH tool.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"bac-nexus/internal/app"
	"bac-nexus/internal/audit"
	"bac-nexus/internal/credential"
	"bac-nexus/internal/mcp"
	"bac-nexus/internal/security"
)

func main() {
	err := runCommand(os.Args[1:], os.Stderr)
	if errors.Is(err, flag.ErrHelp) {
		return
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "nexus:", err)
		os.Exit(1)
	}
}

// mainDeps groups every dependency that the main package needs to
// compose the catalog-context service and the MCP server. Test
// code substitutes fakes; production code supplies the canonical
// implementations from the corresponding internal packages.
type mainDeps struct {
	Profile       string
	Credentials   credential.CredentialStore
	Authorizer    security.Authorizer
	Auditor       audit.Auditor
	Resolver      app.CatalogResolver
	Acquirer      app.SnapshotAcquirer
	Recovery      app.RecoveryCoordinator
	Leases        app.LeaseStore
	ServerFactory func(s *service) (runner, error)
	Now           func() time.Time
}

// service wraps the app.Service with the mcp-facing surface. It
// exists so the main package can hold a typed reference to both
// the local-OS-principal service and the MCP adapter and so the
// composition root has a single, testable unit.
type service struct {
	app     *app.Service
	server  *mcp.Server
	profile string
}

// runner is the minimal surface the MCP server exposes to the main
// package. The production MCP server implements it; tests can
// substitute a deterministic double.
type runner interface {
	Run(ctx context.Context) error
}

// runWithDeps is the composition root used by the serve
// subcommand and by main_test. It builds the app service, invokes
// the pre-acquire recovery gate, constructs the MCP server, and
// runs the server over the supplied transport. A failed startup
// or cancelled context aborts the lifecycle before the server runs.
func runWithDeps(ctx context.Context, deps mainDeps) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(deps.Profile) == "" {
		return errors.New("nexus serve requires a non-empty profile")
	}
	if deps.ServerFactory == nil {
		deps.ServerFactory = newMCPServer
	}
	if deps.Now == nil {
		deps.Now = time.Now
	}
	svc := app.NewService(app.ServiceDeps{
		Credentials: deps.Credentials, Authorizer: deps.Authorizer, Auditor: deps.Auditor,
		Resolver: deps.Resolver, Acquirer: deps.Acquirer, Leases: deps.Leases,
		Recovery: deps.Recovery, Profile: deps.Profile, Now: deps.Now,
	})
	if err := svc.Startup(ctx); err != nil {
		return fmt.Errorf("startup: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	r, err := deps.ServerFactory(&service{app: svc, profile: deps.Profile})
	if err != nil {
		return fmt.Errorf("build mcp server: %w", err)
	}
	return r.Run(ctx)
}

// newMCPServer is the production ServerFactory. It builds the
// canonical typed MCP server over the wrapped app service.
func newMCPServer(s *service) (runner, error) {
	server, err := mcp.New(mcp.Config{
		Info:    mcp.Info{Name: "bac-nexus", Version: "v0.0.0"},
		Service: s.app,
	})
	if err != nil {
		return nil, err
	}
	s.server = server
	return server, nil
}

// defaultDeps returns the canonical production dependency set. The
// serve subcommand supplies the profile from flags; the resolver
// and acquirer are intentionally nil in v1 MCP because the surface
// is restricted to the two read-only tools.
func defaultDeps() mainDeps {
	return mainDeps{
		Credentials:   credential.NewNativeCredentialStore(),
		Authorizer:    security.NewPolicy(),
		Auditor:       audit.NewRecorder(),
		ServerFactory: newMCPServer,
		Now:           time.Now,
	}
}

// runCommand is the CLI entry point. It accepts "serve" and "help"
// subcommands; anything else is rejected so an operator can never
// accidentally invoke a tool that does not exist.
func runCommand(args []string, out io.Writer) error {
	if out == nil {
		out = io.Discard
	}
	if len(args) > 0 {
		switch args[0] {
		case "-h", "--help", "help":
			return printHelp(args[1:], out)
		}
		if strings.HasPrefix(args[0], "-") {
			fmt.Fprintln(out, "root flags are not accepted; use a subcommand")
			return errors.New("root flag is invalid")
		}
	}
	if len(args) == 0 {
		fmt.Fprintln(out, "usage: nexus <subcommand> [flags]")
		fmt.Fprintln(out, "subcommands: serve, help")
		return errors.New("explicit subcommand is required")
	}
	switch args[0] {
	case "serve":
		return runServe(args[1:], out)
	default:
		return fmt.Errorf("unknown subcommand %q", args[0])
	}
}

func printHelp(args []string, out io.Writer) error {
	if len(args) > 0 && args[0] == "serve" {
		return printServeHelp(out)
	}
	fmt.Fprintln(out, "nexus — BAC Nexus v1 MCP stdio server")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "usage: nexus <subcommand> [flags]")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "subcommands:")
	fmt.Fprintln(out, "  serve    Run the typed stdio MCP server with the two allowed tools.")
	fmt.Fprintln(out, "  help     Show this help text or the help for a subcommand.")
	return flag.ErrHelp
}

func printServeHelp(out io.Writer) error {
	fmt.Fprintln(out, "nexus serve — run the typed stdio MCP server")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "usage: nexus serve -profile <name> [flags]")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "flags:")
	fmt.Fprintln(out, "  -profile string   Approved Nexus profile name. Required.")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "tools exposed:")
	fmt.Fprintln(out, "  resolve_catalog_candidates   Resolve up to 50 catalog candidates for a bounded query.")
	fmt.Fprintln(out, "  read_selected_source         Read a single source page for the exact selection bound to a cursor.")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Only the two tools above are exposed. No generic surface is offered.")
	return flag.ErrHelp
}

func runServe(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(out)
	registerServeFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	profile := fs.Lookup("profile").Value.String()
	if strings.TrimSpace(profile) == "" {
		fmt.Fprintln(out, "nexus serve requires -profile <name>")
		return errors.New("serve requires a non-empty profile")
	}
	deps := defaultDeps()
	deps.Profile = profile
	return runWithDeps(context.Background(), deps)
}

func registerServeFlags(fs *flag.FlagSet) {
	fs.String("profile", "", "approved Nexus profile name (required)")
}
