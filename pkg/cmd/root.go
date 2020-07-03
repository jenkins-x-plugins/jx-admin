package cmd

import (
	"github.com/jenkins-x/jx-admin/pkg/cmd/create"
	"github.com/jenkins-x/jx-admin/pkg/cmd/operator"
	"github.com/jenkins-x/jx-admin/pkg/cmd/plugins"
	"github.com/jenkins-x/jx-admin/pkg/cmd/upgrade"
	"github.com/jenkins-x/jx-admin/pkg/cmd/version"
	"github.com/jenkins-x/jx-admin/pkg/rootcmd"
	"github.com/jenkins-x/jx-helpers/pkg/cobras"
	"github.com/jenkins-x/jx-logging/pkg/log"
	"github.com/spf13/cobra"
)

// Main creates the new command
func Main() *cobra.Command {
	cmd := &cobra.Command{
		Use:   rootcmd.TopLevelCommand,
		Short: "commands for creating and upgrading Jenkins X environments using GitOps",
		Run: func(cmd *cobra.Command, args []string) {
			err := cmd.Help()
			if err != nil {
				log.Logger().Errorf(err.Error())
			}
		},
	}
	cmd.AddCommand(cobras.SplitCommand(create.NewCmdCreate()))
	cmd.AddCommand(cobras.SplitCommand(operator.NewCmdOperator()))
	cmd.AddCommand(cobras.SplitCommand(upgrade.NewCmdUpgrade()))
	cmd.AddCommand(cobras.SplitCommand(version.NewCmdVersion()))
	cmd.AddCommand(plugins.NewCmdPlugins())
	return cmd
}
