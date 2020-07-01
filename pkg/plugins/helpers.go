package plugins

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	jenkinsv1 "github.com/jenkins-x/jx-api/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx-helpers/pkg/extensions"
	"github.com/jenkins-x/jx-helpers/pkg/homedir"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// HelmPluginName the default name of the helm plugin
	HelmPluginName = "helm"

	// HelmfilePluginName the name of the helmfile plugin
	HelmfilePluginName = "helmfile"

	// HelmAnnotatePluginName the name of the helmAnnotate plugin
	HelmAnnotatePluginName = "helm-annotate"
)

// GetHelmBinary returns the path to the locally installed helm 3 extension
func GetHelmBinary(version string) (string, error) {
	if version == "" {
		version = HelmVersion
	}
	pluginBinDir, err := getPluginBinDir()
	if err != nil {
		return "", err
	}
	plugin := CreateHelmPlugin(version)
	return extensions.EnsurePluginInstalled(plugin, pluginBinDir)
}

// CreateHelmPlugin creates the helm 3 plugin
func CreateHelmPlugin(version string) jenkinsv1.Plugin {
	binaries := extensions.CreateBinaries(func(p extensions.Platform) string {
		return fmt.Sprintf("https://get.helm.sh/helm-v%s-%s-%s.%s", version, strings.ToLower(p.Goos), strings.ToLower(p.Goarch), p.Extension())
	})

	plugin := jenkinsv1.Plugin{
		ObjectMeta: metav1.ObjectMeta{
			Name: HelmPluginName,
		},
		Spec: jenkinsv1.PluginSpec{
			SubCommand:  "helm",
			Binaries:    binaries,
			Description: "helm 3 binary",
			Name:        HelmPluginName,
			Version:     version,
		},
	}
	return plugin
}

// GetHelmfileBinary returns the path to the locally installed helmfile extension
func GetHelmfileBinary(version string) (string, error) {
	if version == "" {
		version = HelmfileVersion
	}
	pluginBinDir, err := getPluginBinDir()
	if err != nil {
		return "", err
	}
	plugin := CreateHelmfilePlugin(version)
	return extensions.EnsurePluginInstalled(plugin, pluginBinDir)
}

// CreateHelmfilePlugin creates the helm 3 plugin
func CreateHelmfilePlugin(version string) jenkinsv1.Plugin {
	binaries := extensions.CreateBinaries(func(p extensions.Platform) string {
		answer := fmt.Sprintf("https://github.com/roboll/helmfile/releases/download/v%s/helmfile_%s_%s", version, strings.ToLower(p.Goos), strings.ToLower(p.Goarch))
		if p.IsWindows() {
			answer += ".exe"
		}
		return answer
	})

	plugin := jenkinsv1.Plugin{
		ObjectMeta: metav1.ObjectMeta{
			Name: HelmfilePluginName,
		},
		Spec: jenkinsv1.PluginSpec{
			SubCommand:  "helmfile",
			Binaries:    binaries,
			Description: "helmfile  binary",
			Name:        HelmfilePluginName,
			Version:     version,
		},
	}
	return plugin
}

// GetHelmAnnotateBinary returns the path to the locally installed helmAnnotate extension
func GetHelmAnnotateBinary(version string, user string, token string) (string, error) {
	if version == "" {
		version = HelmAnnotateVersion
	}
	pluginBinDir, err := getPluginBinDir()
	if err != nil {
		return "", err
	}
	plugin := CreateHelmAnnotatePlugin(version, user, token)
	aliasFileName := "ha.tar.gz"
	if runtime.GOOS == "windows" {
		aliasFileName = "ha.zip"
	}
	return extensions.EnsurePluginInstalledForAliasFile(plugin, aliasFileName, pluginBinDir)
}

// CreateHelmAnnotatePlugin creates the helm 3 plugin
func CreateHelmAnnotatePlugin(version string, user string, token string) jenkinsv1.Plugin {
	binaries := extensions.CreateBinaries(func(p extensions.Platform) string {
		// darwin
		asset := "20231728"
		switch strings.ToLower(p.Goos) {
		case "linux":
			asset = "20231730"
		case "windows":
			asset = "20231731"
		}
		return fmt.Sprintf("https://%s:%s@api.github.com/repos/jenkins-x/helm-annotate/releases/assets/%s", user, token, asset)
		//return fmt.Sprintf("https://%s:%s@api.github.com/repos/jenkins-x/helm-annotate/releases/assets/v%s/helm-annotate-%s-%s.%s", user, token, version, strings.ToLower(p.Goos), strings.ToLower(p.Goarch), p.Extension())
	})

	plugin := jenkinsv1.Plugin{
		ObjectMeta: metav1.ObjectMeta{
			Name: HelmAnnotatePluginName,
		},
		Spec: jenkinsv1.PluginSpec{
			SubCommand:  "helm-annotate",
			Binaries:    binaries,
			Description: "helm annotate binary",
			Name:        HelmAnnotatePluginName,
			Version:     version,
		},
	}
	return plugin
}

func getPluginBinDir() (string, error) {
	pluginBinDir, err := homedir.PluginBinDir(os.Getenv("JX_REMOTE_HOME"), ".jx-admin")
	if err != nil {
		return "", errors.Wrapf(err, "failed to find plugin home dir")
	}
	return pluginBinDir, nil
}
