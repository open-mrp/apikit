// Command txaudit fails when a database transaction callback has an effect a rollback would not undo, which the transaction manager's deadlock retry would repeat. Run it in CI:
//
//	go run github.com/open-mrp/apikit/cmd/txaudit -root . -external paymentClient,searchClient
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/open-mrp/apikit/txaudit"
)

// errFindings reports that the audit found effects; they are already printed.
var errFindings = errors.New("transaction callbacks have effects a rollback would not undo")

func main() {
	if err := Run(os.Args, os.Stdout, os.Stderr); err != nil {
		if !errors.Is(err, errFindings) && !errors.Is(err, flag.ErrHelp) {
			fmt.Fprintln(os.Stderr, "txaudit:", err)
		}
		os.Exit(1)
	}
}

// Run audits the tree named by -root and prints the findings.
func Run(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "directory to scan")
	external := flags.String("external", "", "comma-separated receiver names that leave the database, added to the defaults")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}

	var cfg txaudit.Config
	for _, name := range strings.Split(*external, ",") {
		if name = strings.TrimSpace(name); name != "" {
			cfg.ExternalReceivers = append(cfg.ExternalReceivers, name)
		}
	}
	findings, callbacks, err := txaudit.Audit(*root, cfg)
	if err != nil {
		return err
	}
	if len(findings) == 0 {
		fmt.Fprintf(stdout, "txaudit: %d transaction callbacks, all confined to the database\n", callbacks)
		return nil
	}
	fmt.Fprintf(stderr, "txaudit: %d transaction callbacks, %d with effects a rollback would not undo\n\n", callbacks, len(findings))
	for _, f := range findings {
		fmt.Fprintf(stderr, "  %s\n    %s: %s\n    %s\n\n", f.Pos, f.Kind, f.Detail, f.Why)
	}
	return errFindings
}
