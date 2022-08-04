package main

import (
	"context"
	"fmt"
	"os"

	"github.com/galaxyed/x/cloudx"

	"github.com/pkg/errors"

	"github.com/spf13/cobra"

	"github.com/galaxyed/x/cmdx"
)

func NewRootCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "ory",
		Short: "The ORY CLI",
	}

	c.AddCommand(
		cloudx.NewProxyCommand("", ""),
		cloudx.NewTunnelCommand("", ""),
	)

	return c
}

func main() {
	rootCmd := NewRootCmd()
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		if !errors.Is(err, cmdx.ErrNoPrintButFail) {
			_, _ = fmt.Fprintln(rootCmd.ErrOrStderr(), err)
		}
		os.Exit(1)
	}
}
