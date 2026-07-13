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
	"github.com/dimatitov/vpn-bypass/internal/config"
	"github.com/dimatitov/vpn-bypass/internal/logstore"
	"github.com/dimatitov/vpn-bypass/internal/service"
)

var (
	version            = "dev"
	commit             = "unknown"
	date               = "unknown"
	ensureInstallation = config.Ensure
	purgeInstallation  = config.Purge
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

	case "list", "sync", "clear", "doctor":
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
		case "doctor":
			if len(args) != 1 {
				return fmt.Errorf("usage: vpn-bypass doctor")
			}
			return application.Doctor(ctx)
		}

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
		paths, err := config.Paths()
		if err != nil {
			return err
		}
		logFile, err := logstore.Open(paths.Logs)
		if err != nil {
			return err
		}
		defer logFile.Close()
		application, err := app.NewWithWriters(io.MultiWriter(out, logFile), io.MultiWriter(errOut, logFile))
		if err != nil {
			return err
		}
		return application.Watch(ctx, *interval)

	case "install":
		if len(args) != 1 {
			return fmt.Errorf("usage: vpn-bypass install")
		}
		return runInstall(ctx, out, newService)

	case "uninstall":
		fs := flag.NewFlagSet("uninstall", flag.ContinueOnError)
		fs.SetOutput(errOut)
		purge := fs.Bool("purge", false, "remove configuration, state, and logs")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return fmt.Errorf("usage: vpn-bypass uninstall [--purge]")
		}
		return runUninstall(ctx, out, newService, newApp, *purge)

	case "status":
		if len(args) != 1 {
			return fmt.Errorf("usage: vpn-bypass status")
		}
		return runStatus(ctx, out, newService, newApp)

	case "logs":
		fs := flag.NewFlagSet("logs", flag.ContinueOnError)
		fs.SetOutput(errOut)
		follow := fs.Bool("follow", false, "follow appended log output")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return fmt.Errorf("usage: vpn-bypass logs [--follow]")
		}
		paths, err := config.Paths()
		if err != nil {
			return err
		}
		return logstore.Show(ctx, logstore.Path(paths.Logs), out, *follow)

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
		return runInstallWithManager(ctx, out, manager)
	case "uninstall":
		return runUninstallWithManager(ctx, out, manager, func() error {
			application, appErr := newApp()
			if appErr != nil {
				return appErr
			}
			return application.Clear(ctx)
		}, false)
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

func runInstall(ctx context.Context, out io.Writer, newService serviceFactory) error {
	manager, err := newService()
	if err != nil {
		return err
	}
	return runInstallWithManager(ctx, out, manager)
}

func runInstallWithManager(ctx context.Context, out io.Writer, manager service.Manager) error {
	paths, err := ensureInstallation()
	if err != nil {
		return fmt.Errorf("prepare installation: %w", err)
	}
	if err := manager.Install(ctx); err != nil {
		return err
	}
	fmt.Fprintln(out, "vpn-bypass installed and started.")
	fmt.Fprintln(out, "Configuration:", paths.Config)
	fmt.Fprintln(out, "Next steps: run 'vpn-bypass status' and 'vpn-bypass doctor'.")
	return nil
}

func runUninstall(ctx context.Context, out io.Writer, newService serviceFactory, newApp func() (*app.App, error), purge bool) error {
	manager, err := newService()
	if err != nil {
		return err
	}
	return runUninstallWithManager(ctx, out, manager, func() error {
		application, appErr := newApp()
		if appErr != nil {
			return appErr
		}
		return application.Clear(ctx)
	}, purge)
}

func runUninstallWithManager(ctx context.Context, out io.Writer, manager service.Manager, clearRoutes func() error, purge bool) error {
	removal, err := uninstallService(ctx, manager, clearRoutes, purge, purgeInstallation)
	if err != nil {
		return err
	}
	fmt.Fprintln(out, "vpn-bypass uninstalled.")
	if purge {
		fmt.Fprintln(out, "Configuration, state, and logs removed.")
	} else {
		fmt.Fprintln(out, "Configuration and logs preserved.")
	}
	if removal.RebootRequired {
		fmt.Fprintln(out, "Installed executable removal is scheduled for the next reboot.")
	}
	return nil
}

func runStatus(ctx context.Context, out io.Writer, newService serviceFactory, newApp func() (*app.App, error)) error {
	manager, err := newService()
	if err != nil {
		return err
	}
	serviceState, err := manager.Status(ctx)
	if err != nil {
		return err
	}
	application, err := newApp()
	if err != nil {
		return err
	}
	status, err := application.Status(ctx)
	if err != nil {
		return err
	}
	lastSync := "never"
	if !status.LastSync.IsZero() {
		lastSync = status.LastSync.UTC().Format(time.RFC3339)
	}
	writeStatus(out, serviceState, status, lastSync)
	return nil
}

func writeStatus(out io.Writer, serviceState service.State, status app.StatusInfo, lastSync string) {
	fmt.Fprintf(out, "version: %s\nservice: %s\ndirect_gateway: %s\ndirect_interface: %s\nmanaged_routes: %d\nlast_successful_sync: %s\nconfig_path: %s\nstate_path: %s\n",
		version, serviceState, status.Gateway, status.Interface, status.Routes, lastSync, status.ConfigPath, status.StatePath)
}

func uninstallService(ctx context.Context, manager service.Manager, clearRoutes func() error, purge bool, purgeData func() error) (service.Removal, error) {
	var errs []error
	if err := manager.Stop(ctx); err != nil {
		errs = append(errs, fmt.Errorf("stop service: %w", err))
	}
	clearErr := clearRoutes()
	if clearErr != nil {
		errs = append(errs, fmt.Errorf("clear owned routes: %w", clearErr))
	}
	removal, err := manager.Remove(ctx)
	if err != nil {
		errs = append(errs, fmt.Errorf("remove service: %w", err))
	}
	if purge {
		if len(errs) != 0 {
			errs = append(errs, fmt.Errorf("purge skipped because uninstall cleanup did not complete"))
		} else if err := purgeData(); err != nil {
			errs = append(errs, fmt.Errorf("purge application data: %w", err))
		}
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
  status                  show application and routing status
  doctor                  check the direct gateway and routing
  watch --interval 60s    continuously refresh routes
  install                 install and start vpn-bypass
  uninstall [--purge]     uninstall and optionally remove all data
  logs [--follow]         show or follow service logs
  service install         install and start the background service
  service uninstall       backward-compatible uninstall alias
  service status          show the background service status
  version                 show build information`)
}
