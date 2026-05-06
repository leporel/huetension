// huetension is a single-binary CLI: subcommands for palette extraction,
// harmony / gradient generation, contrast checks, CVD simulation, and
// export. The actual command tree lives in the cli/ subpackage; this file
// is the entry point only.
package main

import "github.com/leporel/huetension/cmd/huetension/cli"

// version is overridden via -ldflags "-X main.version=…" at release time.
var version = "0.1.0-dev"

func main() {
	cli.Execute(version)
}
