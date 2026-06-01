// Command stardust is the entry point for the Stardust context engine.
//
// It delegates immediately to internal/cli, keeping the binary's main package
// free of logic so the same core is reachable from every surface.
package main

import "github.com/alxxpersonal/stardust/internal/cli"

func main() {
	cli.Execute()
}
