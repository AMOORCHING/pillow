package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/AMOORCHING/pillow/internal/agent"
	"github.com/AMOORCHING/pillow/internal/config"
	"github.com/AMOORCHING/pillow/internal/daemon"
	"github.com/AMOORCHING/pillow/internal/interrupt"
	"github.com/AMOORCHING/pillow/internal/ipc"
	"github.com/AMOORCHING/pillow/internal/privacy"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "pillow",
		Short: "Voice-narrated agentic coding supervision daemon",
		Long: `pillow is a background daemon that supervises agentic coding tools.
It provides voice narration of critical events, irreversibility warnings,
slap-to-negotiate interrupts, and retrospective summaries.

Start the daemon, then use Claude Code (or other tools) with pillow's plugin.`,
		RunE: runDaemon,
	}

	rootCmd.Flags().String("config", "", "path to config file")
	rootCmd.Flags().String("socket-path", "", "override socket path")
	rootCmd.Flags().Bool("verbose", false, "verbose logging")

	setupCmd := &cobra.Command{
		Use:   "setup",
		Short: "Run the interactive setup wizard",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := config.RunWizard()
			return err
		},
	}

	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Print current config",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := config.ConfigPath()
			if !config.Exists() {
				fmt.Printf("No config file found. Run 'pillow setup' first, or create %s manually.\n", path)
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			fmt.Print(string(data))
			return nil
		},
	}

	editConfigCmd := &cobra.Command{
		Use:   "edit",
		Short: "Open config file in $EDITOR",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := config.ConfigPath()
			if !config.Exists() {
				fmt.Printf("No config file found. Run 'pillow setup' first.\n")
				return nil
			}
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "nano"
			}
			c := exec.Command(editor, path)
			c.Stdin = os.Stdin
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			return c.Run()
		},
	}
	configCmd.AddCommand(editConfigCmd)

	recapCmd := &cobra.Command{
		Use:   "recap",
		Short: "Request retrospective summary of current session",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			client := ipc.NewClient(cfg.SocketPath)
			if !client.Ping() {
				return fmt.Errorf("pillow daemon is not running — start with: pillow")
			}
			ctx := context.Background()
			resp, err := client.GetSummary(ctx)
			if err != nil {
				return err
			}
			if resp.Summary == "" {
				fmt.Println("No session summary available yet.")
				return nil
			}
			fmt.Println(resp.Summary)
			return nil
		},
	}

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Print daemon status and current session info",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			client := ipc.NewClient(cfg.SocketPath)
			if !client.Ping() {
				fmt.Println("pillow daemon is not running")
				return nil
			}
			ctx := context.Background()
			resp, err := client.GetStatus(ctx)
			if err != nil {
				return err
			}
			fmt.Printf("pillow daemon running\n")
			if resp.ActiveSession != "" {
				fmt.Printf("  session:  %s\n", resp.ActiveSession)
				fmt.Printf("  events:   %d\n", resp.Events)
				fmt.Printf("  cost:     %s\n", resp.Cost)
			} else {
				fmt.Println("  no active session")
			}
			if resp.Negotiation != nil && resp.Negotiation.Active {
				fmt.Printf("  negotiation: active (outcome: %s)\n", resp.Negotiation.Outcome)
			}
			return nil
		},
	}

	historyCmd := &cobra.Command{
		Use:   "history",
		Short: "Print recent interrupt events",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := config.HistoryPath()
			data, err := os.ReadFile(path)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Println("No interrupt history yet.")
					return nil
				}
				return err
			}
			fmt.Print(string(data))
			return nil
		},
	}

	sensordCmd := &cobra.Command{
		Use:   "sensord",
		Short: "Manage the sensor daemon",
	}

	sensordStatusCmd := &cobra.Command{
		Use:   "status",
		Short: "Check if sensord is running",
		Run: func(cmd *cobra.Command, args []string) {
			cfg, _ := config.Load()
			client := ipc.NewClient(cfg.SensordSocketPath)
			if client.Ping() {
				fmt.Println("pillowsensord is running")
			} else {
				fmt.Println("pillowsensord is not running")
				fmt.Println("Start with: sudo pillowsensord")
			}
		},
	}

	sensordStartCmd := &cobra.Command{
		Use:   "start",
		Short: "Start the sensor daemon (requires sudo)",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := exec.Command("sudo", "pillowsensord")
			c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			if err := c.Start(); err != nil {
				return fmt.Errorf("failed to start pillowsensord: %w", err)
			}
			fmt.Printf("pillowsensord started (pid %d)\n", c.Process.Pid)
			return nil
		},
	}

	sensordStopCmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the sensor daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := exec.Command("sudo", "pkill", "-f", "pillowsensord")
			if err := c.Run(); err != nil {
				return fmt.Errorf("failed to stop pillowsensord: %w", err)
			}
			fmt.Println("pillowsensord stopped")
			return nil
		},
	}

	sensordCmd.AddCommand(sensordStatusCmd, sensordStartCmd, sensordStopCmd)
	rootCmd.AddCommand(setupCmd, configCmd, recapCmd, statusCmd, historyCmd, sensordCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runDaemon(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	if !config.Exists() {
		fmt.Println("First time? Run 'pillow setup' to configure.")
		fmt.Println("Starting with defaults...")
		fmt.Println()
	}

	// Override socket path if flag is set
	if sp, _ := cmd.Flags().GetString("socket-path"); sp != "" {
		cfg.SocketPath = sp
	}

	verbose, _ := cmd.Flags().GetBool("verbose")
	if !verbose {
		log.SetOutput(os.Stderr)
	}

	// Build components based on privacy mode
	components, err := privacy.Build(cfg)
	if err != nil {
		return err
	}
	if components.TTS != nil {
		defer components.TTS.Close()
	}

	// Create daemon
	d := daemon.New(cfg, components.TTS, components.Summarizer)

	// Create IPC server
	server := ipc.NewServer(cfg.SocketPath, d)

	// Set up context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("[pillow] shutting down...")
		cancel()
	}()

	// Start sensord client if slap detection is enabled
	if cfg.SlapThreshold > 0 {
		go startSensordClient(ctx, cfg, d)
	}

	fmt.Printf("pillow daemon starting (socket: %s, privacy: %s)\n", cfg.SocketPath, cfg.PrivacyMode)

	// Start IPC server (blocks until context cancelled)
	return server.Start(ctx)
}

func startSensordClient(ctx context.Context, cfg *config.Config, d *daemon.Daemon) {
	if !interrupt.SensordRunning(cfg.SensordSocketPath) {
		log.Println("[pillow] pillowsensord not running — slap detection unavailable")
		return
	}
	log.Println("[pillow] connected to pillowsensord for slap detection")

	client := interrupt.NewAccelClient(cfg.SensordSocketPath, func(evt agent.SlapEvent) {
		d.BufferSlap(evt)
	})
	if err := client.Run(ctx); err != nil {
		log.Printf("[pillow] sensord client error: %v", err)
	}
}

// ensure Daemon implements EventHandler
var _ ipc.EventHandler = (*daemon.Daemon)(nil)
