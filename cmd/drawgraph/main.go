// Command drawgraph is the agent-native CLI for creating, inspecting, formatting,
// extending icon packs, persisting user habits, and exporting DrawGraph diagrams.
package main

import (
	"os"

	"github.com/neko233-com/DrawGraph/internal/agentcli"
)

func main() {
	os.Exit(agentcli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
