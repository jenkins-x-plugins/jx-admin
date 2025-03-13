package cmd

import (
	"github.com/jenkins-x-plugins/jx-admin/pkg/cmd/create"
	"github.com/jenkins-x-plugins/jx-admin/pkg/cmd/invitations"
	"github.com/jenkins-x-plugins/jx-admin/pkg/cmd/joblog"
	"github.com/jenkins-x-plugins/jx-admin/pkg/cmd/operator"
	"github.com/jenkins-x-plugins/jx-admin/pkg/cmd/plugins"
	"github.com/jenkins-x-plugins/jx-admin/pkg/cmd/stop"
	"github.com/jenkins-x-plugins/jx-admin/pkg/cmd/trigger"
	"github.com/jenkins-x-plugins/jx-admin/pkg/cmd/version"
	"github.com/jenkins-x-plugins/jx-admin/pkg/common"
	"github.com/jenkins-x/jx-helpers/v3/pkg/cobras"
	"github.com/jenkins-x/jx-logging/v3/pkg/log"
	"github.com/spf13/cobra"
)

// Main creates the new command
func Main() *cobra.Command {
	cmd := &cobra.Command{
		Annotations: map[string]string{
			cobra.CommandDisplayNameAnnotation: common.TopLevelCommand,
		},
		Short: "commands for creating and upgrading Jenkins X environments using GitOps",
		Run: func(cmd *cobra.Command, args []string) {
			err := cmd.Help()
			if err != nil {
				log.Logger().Error(err.Error())
			}
		},
	}
	cmd.AddCommand(cobras.SplitCommand(create.NewCmdCreate()))
	cmd.AddCommand(cobras.SplitCommand(invitations.NewCmdInvitations()))
	cmd.AddCommand(cobras.SplitCommand(joblog.NewCmdJobLog()))
	cmd.AddCommand(cobras.SplitCommand(operator.NewCmdOperator()))
	cmd.AddCommand(cobras.SplitCommand(stop.NewCmdJobStop()))
	cmd.AddCommand(cobras.SplitCommand(trigger.NewCmdJobTrigger()))
	cmd.AddCommand(cobras.SplitCommand(version.NewCmdVersion()))
	cmd.AddCommand(plugins.NewCmdPlugins())
	return cmd
}
