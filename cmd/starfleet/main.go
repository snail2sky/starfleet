// Starfleet is a modern process management and scheduling platform.
package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/snail2sky/starfleet/internal/api"
	"github.com/snail2sky/starfleet/internal/api/ws"
	"github.com/snail2sky/starfleet/internal/config"
	"github.com/snail2sky/starfleet/internal/engine"
	"github.com/snail2sky/starfleet/internal/event"
	"github.com/snail2sky/starfleet/internal/notify"
	"github.com/snail2sky/starfleet/internal/scheduler"
	"github.com/snail2sky/starfleet/internal/tui"
	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// Shared flags (persistent across subcommands).
var cfgPath string

// serve-only flags.
var (
	daemonMode   bool
	logFile      string
	tuiLegacy    bool   // deprecated alias: -tui on the root command
	statusServer string // `status --server` override
)

var rootCmd = &cobra.Command{
	Use:   "starfleet",
	Short: "Starfleet — modern process management & scheduling platform",
	Long: `Starfleet manages long-running processes and scheduled tasks with a
web dashboard, terminal UI, real-time logs, health checks and hot-reload.

Examples:
  starfleet serve -c /etc/starfleet/starfleet.yml   # run the server
  starfleet tui                                     # terminal UI (connects to server if up)
  starfleet validate -c starfleet.yml               # check config and exit
  starfleet status                                  # query a running server
  starfleet version                                 # print version`,
	// Root without a subcommand = serve (keeps `starfleet -config x` working).
	RunE:              runServe,
	SilenceUsage:      true,
	SilenceErrors:     false,
	ValidArgsFunction: cobra.NoFileCompletions,
}

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Start the terminal UI",
	Long: `Starts the interactive terminal UI.

If a starfleet server is reachable (per --config), the TUI connects to it in
remote mode — it will not start its own processes. Otherwise it runs standalone.`,
	RunE: runTUI,
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the API server (this is also the default)",
	RunE:  runServe,
}

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate the configuration files and exit",
	RunE:  runValidate,
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the status of a running starfleet server",
	RunE:  runStatus,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("starfleet %s (commit: %s, built: %s)\n", version, commit, date)
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgPath, "config", "c", "starfleet.yml", "path to configuration file")
	rootCmd.Flags().BoolVarP(&daemonMode, "daemon", "d", false, "run as background daemon (logs to file)")
	rootCmd.Flags().StringVar(&logFile, "log", "", "log file path (default: stdout; auto-set in daemon/TUI mode)")
	rootCmd.Flags().BoolVar(&tuiLegacy, "tui", false, "deprecated: use the `tui` subcommand")

	tuiCmd.Flags().StringVar(&logFile, "log", "", "log file path (default: <config dir>/starfleet-tui.log)")
	statusCmd.Flags().StringVar(&statusServer, "server", "", "server URL, e.g. http://192.168.1.100:8099 (default: from config)")

	// serve duplicates the root flags so `starfleet serve -d --log x` works
	// (root-local flags are not inherited by subcommands).
	serveCmd.Flags().BoolVarP(&daemonMode, "daemon", "d", false, "run as background daemon (logs to file)")
	serveCmd.Flags().StringVar(&logFile, "log", "", "log file path (default: stdout; auto-set in daemon mode)")

	// Subcommands — without this, `starfleet version` etc. fall through to serve.
	rootCmd.AddCommand(serveCmd, tuiCmd, validateCmd, statusCmd, versionCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// resolveLogPath picks the log destination.
// Priority: --log > daemon default > TUI default (file, never stdout —
// stdout belongs to the TUI renderer) > stdout.
func resolveLogPath(tui bool) string {
	if logFile != "" {
		return logFile
	}
	if daemonMode {
		return "/var/log/starfleet.log"
	}
	if tui {
		return filepath.Join(filepath.Dir(cfgPath), "starfleet-tui.log")
	}
	return ""
}

// setupLogger configures structured logging and returns the writer in use.
func setupLogger(path string) *os.File {
	if path == "" {
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
		return nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot open log file %s: %v\n", path, err)
		os.Exit(1)
	}
	if daemonMode {
		// Daemon: redirect stdout/stderr too (child output, panics, etc).
		os.Stdout = f
		os.Stderr = f
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(f, &slog.HandlerOptions{Level: slog.LevelInfo})))
	return f
}

// runServe starts the API server and all managed processes.
func runServe(cmd *cobra.Command, args []string) error {
	// Deprecated -tui flag on the root command.
	if tuiLegacy {
		return runTUI(cmd, args)
	}

	logPath := resolveLogPath(false)
	setupLogger(logPath)

	slog.Info("starfleet starting", "version", version, "config", cfgPath, "mode", modeName(false, daemonMode))

	cfg, err := config.Load(cfgPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		return err
	}

	bus := event.NewBus(256)
	defer bus.Close()

	mgr := engine.NewManager()
	mgr.LoadFromConfig(cfg)

	hub := ws.NewHub()
	hub.Start()
	defer hub.Stop()

	// Stream process log lines to WebSocket clients.
	engine.RegisterLogCallback(func(e engine.LogEntry) {
		hub.Broadcast(ws.Message{Type: ws.MsgLog, Payload: e})
	})

	notifier := notify.NewNotifier(cfg.Notifications, bus)
	notifier.Start()
	defer notifier.Stop()

	sched := scheduler.NewScheduler(bus)
	if err := sched.LoadFromConfig(cfg); err != nil {
		slog.Error("failed to load schedules", "error", err)
		return err
	}
	sched.Start()
	defer sched.Stop()

	// reloadConfig reloads all config files and applies changes live.
	reloadConfig := func() error {
		newCfg, err := config.Load(cfgPath)
		if err != nil {
			slog.Error("config reload failed", "error", err)
			return err
		}
		bus.Publish(event.Event{Type: event.EventConfigReloaded})
		slog.Info("config reloaded", "processes", len(newCfg.Processes), "schedules", len(newCfg.Schedules))
		return mgr.ReloadConfig(newCfg)
	}

	// Config watcher: hot-reload every 5s on file change.
	watcher := config.NewWatcher(cfgPath, 5_000_000_000, func(newCfg *config.Config) error {
		bus.Publish(event.Event{Type: event.EventConfigReloaded})
		return mgr.ReloadConfig(newCfg)
	})
	watcher.Start()
	defer watcher.Stop()

	mgr.StartAll()

	router := api.NewRouter(mgr, sched, hub, cfg, reloadConfig)
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	go func() {
		slog.Info("api server starting", "addr", addr)
		if err := router.Run(addr); err != nil {
			slog.Error("api server error", "error", err)
		}
	}()

	if logPath == "" {
		printBanner(cfg)
	}

	// Signals: SIGHUP = reload config, SIGINT/SIGTERM = graceful stop.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	for {
		sig := <-sigCh
		if sig == syscall.SIGHUP {
			slog.Info("received SIGHUP, reloading configuration")
			if err := reloadConfig(); err != nil {
				slog.Error("SIGHUP reload failed", "error", err)
			}
			continue
		}
		slog.Info("received signal, shutting down", "signal", sig)
		break
	}

	mgr.StopAll()
	slog.Info("starfleet stopped")
	if logPath == "" {
		fmt.Println("starfleet stopped.")
	}
	return nil
}

// runTUI starts the terminal UI: remote mode when a server is reachable,
// standalone mode otherwise (never both managing the same processes).
func runTUI(cmd *cobra.Command, args []string) error {
	setupLogger(resolveLogPath(true))
	slog.Info("starfleet starting", "version", version, "config", cfgPath, "mode", "tui")

	cfg, err := config.Load(cfgPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		return err
	}

	apiURL := fmt.Sprintf("http://localhost:%d", cfg.Server.Port)
	var mgr tui.Manager
	if tui.ProbeServer(apiURL) {
		slog.Info("tui: connecting to running starfleet server", "api", apiURL)
		mgr = tui.NewRemoteManager(apiURL)
	} else {
		slog.Info("tui: no server found, running in standalone mode")
		local := engine.NewManager()
		local.LoadFromConfig(cfg)
		local.StartAll()
		defer local.StopAll()
		mgr = tui.NewLocalManager(local)
	}
	if err := tui.RunTUI(mgr, apiURL); err != nil {
		slog.Error("tui error", "error", err)
		return err
	}
	return nil
}

// runValidate loads and validates configuration, printing a summary.
func runValidate(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ invalid configuration: %v\n", err)
		return err
	}
	fmt.Printf("✓ configuration is valid: %s\n", cfgPath)
	fmt.Printf("  server:    %s:%d\n", cfg.Server.Host, cfg.Server.Port)
	fmt.Printf("  auth:      %v\n", cfg.Server.Auth.Enabled)
	fmt.Printf("  processes: %d\n", len(cfg.Processes))
	for name, p := range cfg.Processes {
		status := "auto"
		if !p.Autostart {
			status = "manual"
		}
		fmt.Printf("    - %-24s %-7s autorestart=%s\n", name, status, p.Autorestart)
	}
	fmt.Printf("  schedules: %d\n", len(cfg.Schedules))
	for name, s := range cfg.Schedules {
		state := "enabled"
		if !s.Enabled {
			state = "paused"
		}
		fmt.Printf("    - %-24s %-7s %s\n", name, state, s.Cron)
	}
	return nil
}

// runStatus queries a running starfleet server and prints process status.
func runStatus(cmd *cobra.Command, args []string) error {
	base := statusServer
	if base == "" {
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return fmt.Errorf("load config (or pass --server): %w", err)
		}
		host := cfg.Server.Host
		if host == "0.0.0.0" || host == "" {
			host = "127.0.0.1"
		}
		base = fmt.Sprintf("http://%s:%d", host, cfg.Server.Port)
	}

	mgr := tui.NewRemoteManager(base)
	procs := mgr.List()
	if len(procs) == 0 {
		return fmt.Errorf("no response from %s (is starfleet running?)", base)
	}

	fmt.Printf("starfleet at %s\n\n", base)
	fmt.Printf("%-24s %-10s %-8s %-14s %-6s\n", "NAME", "STATE", "PID", "UPTIME", "RETRY")
	for _, p := range procs {
		uptime := "—"
		if p.State == "RUNNING" {
			uptime = fmtUptime(p.Uptime)
		}
		fmt.Printf("%-24s %-10s %-8d %-14s %-6d\n", p.Name, p.State, p.PID, uptime, p.RetryCount)
	}
	return nil
}

// fmtUptime formats Go duration strings like "8m29.614256022s" as "8m29s".
func fmtUptime(s string) string {
	d, err := time.ParseDuration(s)
	if err != nil {
		return s
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	default:
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	}
}

func modeName(tui, daemon bool) string {
	if tui {
		return "tui"
	}
	if daemon {
		return "daemon"
	}
	return "foreground"
}

func printBanner(cfg *config.Config) {
	fmt.Println()
	fmt.Println("  ╔══════════════════════════════════════════════╗")
	fmt.Println("  ║           🚀 Starfleet is running            ║")
	fmt.Println("  ╠══════════════════════════════════════════════╣")
	fmt.Printf("  ║  API:       http://%s:%-22s║\n", cfg.Server.Host, fmt.Sprintf("%d", cfg.Server.Port))
	fmt.Printf("  ║  Config:    %-32s║\n", cfgPath)
	fmt.Printf("  ║  Processes: %-32s║\n", fmt.Sprintf("%d loaded", len(cfg.Processes)))
	fmt.Printf("  ║  Schedules: %-32s║\n", fmt.Sprintf("%d loaded", len(cfg.Schedules)))
	fmt.Println("  ║  Mode:      foreground                       ║")
	fmt.Println("  ║  TUI:       starfleet tui                    ║")
	fmt.Println("  ║  Status:    starfleet status                 ║")
	fmt.Println("  ║  Reload:    SIGHUP or POST /config/reload    ║")
	fmt.Println("  ║  Stop:      Ctrl+C (SIGTERM)                 ║")
	fmt.Println("  ╚══════════════════════════════════════════════╝")
	fmt.Println()
}
