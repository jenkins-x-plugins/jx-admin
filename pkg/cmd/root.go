package cmd

import (
	"github.com/jenkins-x/jx-helpers/pkg/cobras"
	"github.com/jenkins-x/jx-logging/pkg/log"
	"github.com/jenkins-x/jx-remote/pkg/cmd/create"
	"github.com/jenkins-x/jx-remote/pkg/cmd/operator"
	"github.com/jenkins-x/jx-remote/pkg/cmd/upgrade"
	"github.com/jenkins-x/jx-remote/pkg/cmd/version"
	"github.com/jenkins-x/jx-remote/pkg/rootcmd"
	"github.com/spf13/cobra"
)

// Main creates the new command
func Main() *cobra.Command {
	cmd := &cobra.Command{
		Use:   rootcmd.TopLevelCommand,
		Short: "boots up Jenkins and/or Jenkins X in a Kubernetes cluster using GitOps",
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
	return cmd
}
