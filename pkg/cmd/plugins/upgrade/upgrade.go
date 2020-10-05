package upgrade

import (
	"os"
	"path/filepath"

	"github.com/jenkins-x/jx-admin/pkg/plugins"
	"github.com/jenkins-x/jx-helpers/v3/pkg/cmdrunner"
	"github.com/jenkins-x/jx-helpers/v3/pkg/cobras/helper"
	"github.com/jenkins-x/jx-helpers/v3/pkg/cobras/templates"
	"github.com/jenkins-x/jx-logging/v3/pkg/log"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	cmdLong = templates.LongDesc(`
		Upgrades the binary plugins of the secret command (e.g. the helm binary)
`)

	cmdExample = templates.Examples(`
		# upgrades the plugin binaries
		jx upgrade
	`)
)

// UpgradeOptions the options for upgrading a cluster
type Options struct {
	CommandRunner cmdrunner.CommandRunner
	BinDir        string
}

// NewCmdUpgrade creates a command object for the command
func NewCmdUpgrade() (*cobra.Command, *Options) {
	o := &Options{}

	cmd := &cobra.Command{
		Use:     "upgrade",
		Short:   "Upgrades the binary plugins of the secret command (e.g. the Vault binary)",
		Long:    cmdLong,
		Example: cmdExample,
		Run: func(cmd *cobra.Command, args []string) {
			err := o.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&o.BinDir, "bin", "", "", "if set creates a symlink in the bin dir to the plugin binary")

	return cmd, o
}

// Run implements the command
func (o *Options) Run() error {
	log.Logger().Infof("checking we have the correct vault CLI version")
	bin, err := plugins.GetHelmBinary("")
	if err != nil {
		return errors.Wrapf(err, "failed to check vault binary")
	}

	if o.BinDir != "" {
		f := filepath.Join(o.BinDir, "helm")
		err = os.Remove(f)
		if err != nil {
			log.Logger().Warnf("failed to remove %s due to %s", f, err.Error())
		}
		err = os.Symlink(bin, f)
		if err != nil {
			return errors.Wrapf(err, "failed to create symlink from %s to %s", bin, f)
		}
	}
	return nil
}
