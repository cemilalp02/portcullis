// Command portcullis scans AI agent context files for malicious instructions and
// risky auto-execution before an agent reads them.
package main

import (
	"os"

	"github.com/cemilalp02/portcullis/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
