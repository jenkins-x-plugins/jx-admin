package plugins

import (
	"fmt"
	"runtime"
	"strings"

	jenkinsv1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/extensions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Platform represents a platform for binaries
type Platform struct {
	Goarch string
	Goos   string
}

const (
	// HelmPluginName the default name of the helm plugin
	HelmPluginName = "helm"

	// HelmfilePluginName the name of the helmfile plugin
	HelmfilePluginName = "helmfile"

	// HelmAnnotatePluginName the name of the helmAnnotate plugin
	HelmAnnotatePluginName = "helm-annotate"
)

var (
	defaultPlatforms = []Platform{
		{
			Goarch: "amd64",
			Goos:   "Windows",
		},
		{
			Goarch: "amd64",
			Goos:   "Darwin",
		},
		{
			Goarch: "amd64",
			Goos:   "Linux",
		},
		{
			Goarch: "arm",
			Goos:   "Linux",
		},
		{
			Goarch: "386",
			Goos:   "Linux",
		},
	}
)

// Extension returns the default distribution extension; `tar.gz` or `zip` for windows
func (p Platform) Extension() string {
	if p.IsWindows() {
		return "zip"
	}
	return "tar.gz"
}

// IsWindows returns true if the platform is windows
func (p Platform) IsWindows() bool {
	return p.Goos == "Windows"
}

// GetHelmBinary returns the path to the locally installed helm 3 extension
func GetHelmBinary(version string) (string, error) {
	if version == "" {
		version = HelmVersion
	}
	plugin := CreateHelmPlugin(version)
	return extensions.EnsurePluginInstalled(plugin)
}

// CreateHelmPlugin creates the helm 3 plugin
func CreateHelmPlugin(version string) jenkinsv1.Plugin {
	binaries := CreateBinaries(func(p Platform) string {
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
	plugin := CreateHelmfilePlugin(version)
	return extensions.EnsurePluginInstalled(plugin)
}

// CreateHelmfilePlugin creates the helm 3 plugin
func CreateHelmfilePlugin(version string) jenkinsv1.Plugin {
	binaries := CreateBinaries(func(p Platform) string {
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
	plugin := CreateHelmAnnotatePlugin(version, user, token)
	aliasFileName := "ha.tar.gz"
	if runtime.GOOS == "windows" {
		aliasFileName = "ha.zip"
	}
	return extensions.EnsurePluginInstalledForAliasFile(plugin, aliasFileName)
}

// CreateHelmAnnotatePlugin creates the helm 3 plugin
func CreateHelmAnnotatePlugin(version string, user string, token string) jenkinsv1.Plugin {
	binaries := CreateBinaries(func(p Platform) string {
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

// CreateBinaries a helper function to create the binary resources for the platforms for a given callback
func CreateBinaries(createURLFn func(Platform) string) []jenkinsv1.Binary {
	answer := []jenkinsv1.Binary{}
	for _, p := range defaultPlatforms {
		u := createURLFn(p)
		if u != "" {
			answer = append(answer, jenkinsv1.Binary{
				Goarch: p.Goarch,
				Goos:   p.Goos,
				URL:    u,
			})
		}
	}
	return answer
}
