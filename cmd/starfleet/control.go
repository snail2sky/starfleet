package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/snail2sky/starfleet/internal/client"
	"github.com/snail2sky/starfleet/internal/config"
	"github.com/snail2sky/starfleet/internal/engine"
	"github.com/spf13/cobra"
)

// Control subcommands operate on a *running* starfleet server over its API.

var (
	logLines  int
	logFollow bool
	flagUser  string // --user: auth username override
	flagPass  string // --password: auth password override
)

var startCmd = &cobra.Command{
	Use:   "start <process>",
	Short: "Start a managed process",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := resolveClient()
		if err != nil {
			return err
		}
		if err := c.Start(args[0]); err != nil {
			return err
		}
		fmt.Printf("started %s\n", args[0])
		return nil
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop <process>",
	Short: "Stop a managed process",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := resolveClient()
		if err != nil {
			return err
		}
		if err := c.Stop(args[0]); err != nil {
			return err
		}
		fmt.Printf("stopped %s\n", args[0])
		return nil
	},
}

var restartCmd = &cobra.Command{
	Use:   "restart <process>",
	Short: "Restart a managed process",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := resolveClient()
		if err != nil {
			return err
		}
		if err := c.Restart(args[0]); err != nil {
			return err
		}
		fmt.Printf("restarted %s\n", args[0])
		return nil
	},
}

var reloadCmd = &cobra.Command{
	Use:   "reload",
	Short: "Ask the server to hot-reload its configuration",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := resolveClient()
		if err != nil {
			return err
		}
		if err := c.Reload(); err != nil {
			return err
		}
		fmt.Println("configuration reloaded")
		return nil
	},
}

var logsCmd = &cobra.Command{
	Use:   "logs [process]",
	Short: "Show starfleet's own logs, or a process's logs",
	Long: `Show logs from a running starfleet server.

  starfleet logs            starfleet's own server logs
  starfleet logs ttyd       logs of a managed process
  starfleet logs -f -n 500  follow mode (polls every second)`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := resolveClient()
		if err != nil {
			return err
		}
		name := ""
		if len(args) == 1 {
			name = args[0]
		}
		fetch := func() []engine.LogEntry {
			if name == "" {
				return c.ServerLogs(logLines)
			}
			return c.Logs(name, logLines)
		}

		entries := fetch()
		if len(entries) == 0 && !logFollow {
			fmt.Fprintln(os.Stderr, "no log entries (server just started, or wrong name?)")
			return nil
		}
		printLogEntries(entries)

		if !logFollow {
			return nil
		}
		// Follow mode: poll and print new lines until interrupted.
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		var last engine.LogEntry
		if len(entries) > 0 {
			last = entries[len(entries)-1]
		}
		for {
			select {
			case <-sigCh:
				return nil
			case <-time.After(time.Second):
			}
			for _, e := range fetch() {
				if e.Timestamp < last.Timestamp ||
					(e.Timestamp == last.Timestamp && e.Line <= last.Line) {
					continue
				}
				printLogEntries([]engine.LogEntry{e})
				last = e
			}
		}
	},
}

var runCmd = &cobra.Command{
	Use:   "run <schedule>",
	Short: "Trigger a scheduled job immediately",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := resolveClient()
		if err != nil {
			return err
		}
		if err := c.RunSchedule(args[0]); err != nil {
			return err
		}
		fmt.Printf("triggered %s\n", args[0])
		return nil
	},
}

var schedulesCmd = &cobra.Command{
	Use:   "schedules",
	Short: "List scheduled jobs",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := resolveClient()
		if err != nil {
			return err
		}
		jobs, err := c.Schedules()
		if err != nil {
			return err
		}
		if len(jobs) == 0 {
			fmt.Println("no scheduled jobs")
			return nil
		}
		fmt.Printf("%-24s %-14s %-7s %-9s %-20s %s\n", "NAME", "CRON", "TYPE", "ENABLED", "NEXT RUN", "COMMAND/URL")
		for _, j := range jobs {
			cmdOrURL := j.Command
			if cmdOrURL == "" {
				cmdOrURL = j.URL
			}
			next := j.NextRun
			if next == "" {
				next = "—"
			}
			fmt.Printf("%-24s %-14s %-7s %-9t %-20s %s\n", j.Name, j.Cron, j.Type, j.Enabled, next, cmdOrURL)
		}
		return nil
	},
}

// resolveClient builds an API client from --server or the config file.
// Credentials come from --user/--password, then STARFLEET_USER/
// STARFLEET_PASSWORD, then auth.* from the config (config only when the
// server address also comes from the config — never sent to --server).
func resolveClient() (*client.Client, error) {
	var auth config.AuthConfig
	base := serverURL
	if base == "" {
		if err := resolveConfigFlag(); err != nil {
			return nil, fmt.Errorf("load config (or pass --server): %w", err)
		}
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return nil, fmt.Errorf("load config (or pass --server): %w", err)
		}
		host := cfg.Server.Host
		if host == "0.0.0.0" || host == "" {
			host = "127.0.0.1"
		}
		base = fmt.Sprintf("http://%s:%d", host, cfg.Server.Port)
		auth = cfg.Server.Auth
	}
	c := client.New(base)

	user, pass := flagUser, flagPass
	if user == "" {
		user = os.Getenv("STARFLEET_USER")
	}
	if pass == "" {
		pass = os.Getenv("STARFLEET_PASSWORD")
	}
	// Use config credentials whenever they exist — even while auth is
	// disabled on disk: the server may still enforce the old settings
	// until this very reload applies them.
	if user == "" && pass == "" && (auth.Username != "" || auth.Password != "") {
		user, pass = auth.Username, auth.Password
	}
	if user != "" || pass != "" {
		c.SetAuth(user, pass)
	}
	return c, nil
}

// printLogEntries writes entries as: 15:04:05.000 [STREAM] process line
func printLogEntries(entries []engine.LogEntry) {
	for _, e := range entries {
		ts := time.UnixMilli(e.Timestamp).Format("15:04:05.000")
		fmt.Printf("%s [%-5s] %-14s %s\n", ts, e.Stream, e.Process, e.Line)
	}
}

func init() {
	logsCmd.Flags().IntVarP(&logLines, "lines", "n", 100, "number of log lines to show")
	logsCmd.Flags().BoolVarP(&logFollow, "follow", "f", false, "poll for new lines until interrupted")

	// Auth overrides for control commands (default: config, then env).
	rootCmd.PersistentFlags().StringVar(&flagUser, "user", "", "auth username (default: config auth.username or $STARFLEET_USER)")
	rootCmd.PersistentFlags().StringVar(&flagPass, "password", "", "auth password (default: config auth.password or $STARFLEET_PASSWORD)")

	// status's --server lives on the root as a persistent flag; nothing to
	// register here for the other commands.
	rootCmd.AddCommand(startCmd, stopCmd, restartCmd, reloadCmd, logsCmd, runCmd, schedulesCmd)
}
