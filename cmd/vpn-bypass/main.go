package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/dimatitov/vpn-bypass/internal/app"
	"github.com/dimatitov/vpn-bypass/internal/service"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

type serviceFactory func() (service.Manager, error)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr, service.New); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run(args []string, out, errOut io.Writer, newService serviceFactory) error {
	if len(args) == 0 {
		printHelp(out)
		return nil
	}

	command := args[0]
	ctx := context.Background()
	newApp := func() (*app.App, error) { return app.NewWithWriters(out, errOut) }

	switch command {
	case "add":
		if len(args) != 2 {
			return fmt.Errorf("usage: vpn-bypass add <domain|cidr>")
		}
		application, err := newApp()
		if err != nil {
			return err
		}
		return application.Add(args[1])

	case "remove":
		if len(args) != 2 {
			return fmt.Errorf("usage: vpn-bypass remove <domain|cidr>")
		}
		application, err := newApp()
		if err != nil {
			return err
		}
		return application.Remove(args[1])

	case "list", "sync", "clear", "status", "doctor", "watch":
		application, err := newApp()
		if err != nil {
			return err
		}
		switch command {
		case "list":
			if len(args) != 1 {
				return fmt.Errorf("usage: vpn-bypass list")
			}
			return application.List()
		case "sync":
			if len(args) != 1 {
				return fmt.Errorf("usage: vpn-bypass sync")
			}
			return application.Sync(ctx)
		case "clear":
			if len(args) != 1 {
				return fmt.Errorf("usage: vpn-bypass clear")
			}
			return application.Clear(ctx)
		case "status":
			if len(args) != 1 {
				return fmt.Errorf("usage: vpn-bypass status")
			}
			return application.Status()
		case "doctor":
			if len(args) != 1 {
				return fmt.Errorf("usage: vpn-bypass doctor")
			}
			return application.Doctor(ctx)
		case "watch":
			fs := flag.NewFlagSet("watch", flag.ContinueOnError)
			fs.SetOutput(errOut)
			interval := fs.Duration("interval", time.Minute, "route refresh interval")
			if err := fs.Parse(args[1:]); err != nil {
				return err
			}
			if fs.NArg() != 0 {
				return fmt.Errorf("usage: vpn-bypass watch [--interval 60s]")
			}
			return application.Watch(ctx, *interval)
		}

	case "service":
		return runService(ctx, args[1:], out, errOut, newService, newApp)

	case "version":
		if len(args) != 1 {
			return fmt.Errorf("usage: vpn-bypass version")
		}
		fmt.Fprintf(out, "version: %s\ncommit: %s\ndate: %s\n", version, commit, date)
		return nil

	case "help", "-h", "--help":
		if len(args) != 1 {
			return fmt.Errorf("usage: vpn-bypass help")
		}
		printHelp(out)
		return nil

	default:
		return fmt.Errorf("unknown command %q", command)
	}
	return nil
}

func runService(ctx context.Context, args []string, out, _ io.Writer, newService serviceFactory, newApp func() (*app.App, error)) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: vpn-bypass service <install|uninstall|status>")
	}
	if args[0] != "install" && args[0] != "uninstall" && args[0] != "status" {
		return fmt.Errorf("unknown service command %q; expected install, uninstall, or status", args[0])
	}
	manager, err := newService()
	if err != nil {
		return err
	}

	switch args[0] {
	case "install":
		if err := manager.Install(ctx); err != nil {
			return err
		}
		fmt.Fprintln(out, "Service installed and started.")
		return nil
	case "uninstall":
		removal, err := uninstallService(ctx, manager, func() error {
			application, appErr := newApp()
			if appErr != nil {
				return appErr
			}
			return application.Clear(ctx)
		})
		if err != nil {
			return err
		}
		fmt.Fprintln(out, "Service uninstalled.")
		if removal.RebootRequired {
			fmt.Fprintln(out, "Installed executable removal is scheduled for the next reboot.")
		}
		return nil
	case "status":
		state, err := manager.Status(ctx)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "status: %s\n", state)
		return nil
	}
	return nil
}

func uninstallService(ctx context.Context, manager service.Manager, clearRoutes func() error) (service.Removal, error) {
	var errs []error
	if err := manager.Stop(ctx); err != nil {
		errs = append(errs, fmt.Errorf("stop service: %w", err))
	}
	if err := clearRoutes(); err != nil {
		errs = append(errs, fmt.Errorf("clear owned routes: %w", err))
	}
	removal, err := manager.Remove(ctx)
	if err != nil {
		errs = append(errs, fmt.Errorf("remove service: %w", err))
	}
	return removal, errors.Join(errs...)
}

func printHelp(out io.Writer) {
	fmt.Fprintln(out, `vpn-bypass routes selected domains and IP networks outside a full-tunnel VPN

Commands:
  add <domain|cidr>       add a domain, IP address, or network
  remove <domain|cidr>    remove an entry
  list                    show configuration
  sync                    update routes
  clear                   remove routes owned by vpn-bypass
  status                  show saved routing state
  doctor                  check the direct gateway and routing
  watch --interval 60s    continuously refresh routes
  service install         install and start the background service
  service uninstall       stop and uninstall the background service
  service status          show the background service status
  version                 show build information`)
}
