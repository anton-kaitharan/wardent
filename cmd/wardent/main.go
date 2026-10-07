package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/anton-kaitharan/wardent/internal/doctor"
	"github.com/anton-kaitharan/wardent/internal/hook"
	"github.com/anton-kaitharan/wardent/internal/install"
	"github.com/anton-kaitharan/wardent/internal/query"
)

const Version = "0.1.0-phase1"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]

	switch cmd {
	case "install":
		runInstall(os.Args[2:])
	case "uninstall":
		runUninstall(os.Args[2:])
	case "hook":
		runHook(os.Args[2:])
	case "log":
		runLog(os.Args[2:])
	case "explain":
		runExplain(os.Args[2:])
	case "doctor":
		runDoctor(os.Args[2:])
	case "version", "-v", "--version":
		fmt.Printf("wardent version %s\n", Version)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\nRun 'wardent help' for usage.\n", cmd)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Printf(`Wardent — Control and Safety Layer for AI Coding Agents (v%s)

USAGE:
  wardent <command> [arguments...]

COMMANDS:
  install     Install Wardent hooks into Devin CLI configuration
  uninstall   Remove Wardent hooks from Devin CLI configuration
  hook        Internal hook dispatcher (called by the agent harness)
  log         View recorded session and tool events from the local audit log
  explain     Inspect a specific event and show Wardent's safety assessment
  doctor      Diagnose installation integrity, binary resolution, and heartbeat
  version     Display the Wardent version

RUN 'wardent <command> --help' FOR MORE INFORMATION ON A SPECIFIC COMMAND.
`, Version)
}

func runInstall(args []string) {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	userFlag := fs.Bool("user", false, "Install hooks globally in user config")
	dirFlag := fs.String("dir", "", "Project directory for .devin/hooks.v1.json (default: current directory)")
	_ = fs.Parse(args)

	if *userFlag {
		backup, err := install.InstallUser()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error installing user hooks: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Successfully installed Wardent hooks into user config (%s)\n", install.GetUserConfigPath())
		if backup != "" {
			fmt.Printf("Backup saved to: %s\n", backup)
		}
	} else {
		targetDir := *dirFlag
		if targetDir == "" {
			targetDir, _ = os.Getwd()
		}
		backup, err := install.InstallProject(targetDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error installing project hooks: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Successfully installed Wardent hooks into %s\n", install.GetProjectHooksPath(targetDir))
		if backup != "" {
			fmt.Printf("Backup saved to: %s\n", backup)
		}
	}
}

func runUninstall(args []string) {
	fs := flag.NewFlagSet("uninstall", flag.ExitOnError)
	userFlag := fs.Bool("user", false, "Remove hooks globally from user config")
	dirFlag := fs.String("dir", "", "Project directory for .devin/hooks.v1.json (default: current directory)")
	_ = fs.Parse(args)

	if *userFlag {
		if err := install.UninstallUser(); err != nil {
			fmt.Fprintf(os.Stderr, "Error uninstalling user hooks: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Successfully removed Wardent hooks from user config (%s)\n", install.GetUserConfigPath())
	} else {
		targetDir := *dirFlag
		if targetDir == "" {
			targetDir, _ = os.Getwd()
		}
		if err := install.UninstallProject(targetDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error uninstalling project hooks: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Successfully removed Wardent hooks from %s\n", targetDir)
	}
}

func runHook(args []string) {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: wardent hook <agent> <event-slug>")
		os.Exit(0) // always exit 0 to guarantee fail-safe
	}
	agent := args[0]
	eventSlug := args[1]

	code := hook.Handle(agent, eventSlug, os.Stdin, os.Stdout)
	os.Exit(code)
}

func runLog(args []string) {
	fs := flag.NewFlagSet("log", flag.ExitOnError)
	sessFlag := fs.String("session", "", "Filter by session ID")
	toolFlag := fs.String("tool", "", "Filter by tool name (e.g. exec, write, read)")
	limitFlag := fs.Int("limit", 50, "Limit number of records displayed (default: 50)")
	_ = fs.Parse(args)

	records, err := query.ReadAuditRecords("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading audit records: %v\n", err)
		os.Exit(1)
	}

	filtered := query.FilterRecords(records, query.Filter{
		SessionID: *sessFlag,
		ToolName:  *toolFlag,
		Limit:     *limitFlag,
	})

	query.PrintLogTable(filtered, os.Stdout)
}

func runExplain(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: wardent explain <record_id|event_id>")
		os.Exit(1)
	}
	id := args[0]

	records, err := query.ReadAuditRecords("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading audit log: %v\n", err)
		os.Exit(1)
	}

	if err := query.ExplainRecord(records, id, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runDoctor(args []string) {
	ok := doctor.RunDiagnostics(os.Stdout)
	if !ok {
		os.Exit(1)
	}
}
