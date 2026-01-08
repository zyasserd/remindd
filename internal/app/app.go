package app

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"remindd/internal/config"
	"remindd/internal/core"
	"remindd/internal/exitcode"
	"remindd/internal/state"
)

func Run(argv []string, stdout, stderr io.Writer) int {
	if len(argv) < 2 {
		printUsage(stderr)
		return exitcode.ConfigError
	}

	now := time.Now()
	cmd := argv[1]
	switch cmd {
	case "check":
		return runCheck(argv[2:], now, stdout, stderr)
	case "run":
		return runRun(argv[2:], now, stdout, stderr)
	case "snooze":
		return runSnooze(argv[2:], now, stdout, stderr)
	case "list":
		return runList(argv[2:], now, stdout, stderr)
	case "help", "-h", "--help":
		printUsage(stdout)
		return exitcode.OK
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n", cmd)
		printUsage(stderr)
		return exitcode.ConfigError
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "remindd")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  remindd check")
	fmt.Fprintln(w, "  remindd run <name>")
	fmt.Fprintln(w, "  remindd snooze <name> <seconds>")
	fmt.Fprintln(w, "  remindd list")
}

func loadConfigOrExit(stderr io.Writer) (*config.Config, int) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(stderr, "config error: %v\n", err)
		return nil, exitcode.ConfigError
	}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(stderr, "config error: %v\n", err)
		return nil, exitcode.ConfigError
	}
	return cfg, exitcode.OK
}

func runCheck(args []string, now time.Time, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return exitcode.ConfigError
	}

	cfg, code := loadConfigOrExit(stderr)
	if code != exitcode.OK {
		return code
	}

	engine := core.NewEngine(cfg)
	if err := engine.CheckAll(now); err != nil {
		return mapErr(stderr, err)
	}
	return exitcode.OK
}

func runRun(args []string, now time.Time, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return exitcode.ConfigError
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: remindd run <name>")
		return exitcode.ConfigError
	}
	name := fs.Arg(0)

	cfg, code := loadConfigOrExit(stderr)
	if code != exitcode.OK {
		return code
	}

	engine := core.NewEngine(cfg)
	if err := engine.RunAction(now, name); err != nil {
		return mapErr(stderr, err)
	}
	fmt.Fprintf(stdout, "ran %s\n", name)
	return exitcode.OK
}

func runSnooze(args []string, now time.Time, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("snooze", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return exitcode.ConfigError
	}
	if fs.NArg() != 2 {
		fmt.Fprintln(stderr, "usage: remindd snooze <name> <seconds>")
		return exitcode.ConfigError
	}
	name := fs.Arg(0)
	secStr := fs.Arg(1)
	secs, err := strconv.ParseInt(secStr, 10, 64)
	if err != nil || secs <= 0 {
		fmt.Fprintf(stderr, "invalid seconds %q (expected positive integer)\n", secStr)
		return exitcode.ConfigError
	}
	dur := time.Duration(secs) * time.Second

	cfg, code := loadConfigOrExit(stderr)
	if code != exitcode.OK {
		return code
	}

	engine := core.NewEngine(cfg)
	if err := engine.Snooze(now, name, dur); err != nil {
		return mapErr(stderr, err)
	}
	fmt.Fprintf(stdout, "snoozed %s for %ds\n", name, secs)
	return exitcode.OK
}

func runList(args []string, now time.Time, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return exitcode.ConfigError
	}

	cfg, code := loadConfigOrExit(stderr)
	if code != exitcode.OK {
		return code
	}

	// Deterministic output.
	names := make([]string, 0, len(cfg.Reminders))
	for name := range cfg.Reminders {
		names = append(names, name)
	}
	sort.Strings(names)

	engine := core.NewEngine(cfg)
	out := stdout
	var tw *tabwriter.Writer
	if shouldPrintListHeader(stdout) {
		tw = tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
		out = tw
		fmt.Fprintln(out, "NAME\tTYPE\tSTATUS\tFREQ\tINFO")
	}
	for _, name := range names {
		rc := cfg.Reminders[name]
		st, err := state.Load(name)
		if err != nil {
			return mapErr(stderr, err)
		}

		line, err := engine.FormatListLine(now, name, rc, st)
		if err != nil {
			return mapErr(stderr, err)
		}
		fmt.Fprintln(out, strings.TrimRight(line, "\n"))
	}
	if tw != nil {
		_ = tw.Flush()
	}
	return exitcode.OK
}

func shouldPrintListHeader(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func mapErr(stderr io.Writer, err error) int {
	if err == nil {
		return exitcode.OK
	}

	var e *core.ExitError
	if ok := core.AsExitError(err, &e); ok {
		fmt.Fprintln(stderr, e.Message)
		return e.Code
	}

	// Default to config error.
	fmt.Fprintf(stderr, "%v\n", err)
	return exitcode.ConfigError
}
