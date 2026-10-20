package main

import (
	"context"
	"flag"
	"fmt"
	"os"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), "Usage: gh profile\n\nAn interactive dashboard for your authenticated GitHub profile.\n\nScroll: ↑/↓ or j/k, PgUp/PgDn, Home/End. Quit: q or Ctrl-C.")
	}
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "gh-profile: unexpected arguments; use --help")
		os.Exit(1)
	}
	if err := run(context.Background(), os.Stdin, os.Stdout, loadProfile); err != nil {
		fmt.Fprintf(os.Stderr, "gh-profile: %s\n", sanitizeText(err.Error()))
		os.Exit(1)
	}
}
