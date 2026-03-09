package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/smallnest/goclaw/cli"
)

var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
	BuiltBy = "unknown"
)

func main() {
	setupStackDumpSignal()

	cli.SetVersion(Version)

	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}

func dumpGoroutineStacks() {
	buf := make([]byte, 1<<20)
	n := runtime.Stack(buf, true)
	if n == 0 {
		fmt.Fprintln(os.Stderr, "No goroutine stack traces available")
		return
	}

	fmt.Fprintln(os.Stderr, "========== Goroutine Stack Traces ==========")
	fmt.Fprintln(os.Stderr)
	if _, err := os.Stderr.Write(buf[:n]); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing stack traces: %v\n", err)
	}
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "========== End of Stack Traces ==========")
}

func GetVersionInfo() string {
	return fmt.Sprintf("goclaw version %s (commit: %s, built at: %s by: %s)", Version, Commit, Date, BuiltBy)
}
