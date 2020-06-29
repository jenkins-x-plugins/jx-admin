package plugins

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/jenkins-x/jx-logging/pkg/log"
	jenkinsv1 "github.com/jenkins-x/jx/v2/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/v2/pkg/extensions"
	"github.com/jenkins-x/jx/v2/pkg/util"
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
	return EnsurePluginInstalledForAliasFile(plugin, aliasFileName)
}

// EnsurePluginInstalledForAliasFile ensures that the correct version of a plugin is installed locally.
// It will clean up old versions.
func EnsurePluginInstalledForAliasFile(plugin jenkinsv1.Plugin, aliasFileName string) (string, error) {
	pluginBinDir, err := util.PluginBinDir(plugin.ObjectMeta.Namespace)
	if err != nil {
		return "", err
	}
	version := plugin.Spec.Version
	path := filepath.Join(pluginBinDir, fmt.Sprintf("%s-%s", plugin.Spec.Name, version))
	if _, err = os.Stat(path); os.IsNotExist(err) {
		u, err := extensions.FindPluginUrl(plugin.Spec)
		if err != nil {
			return "", err
		}
		log.Logger().Infof("Installing plugin %s version %s for command %s from %s into %s", util.ColorInfo(plugin.Spec.Name),
			util.ColorInfo(version), util.ColorInfo(fmt.Sprintf("jx %s", plugin.Spec.SubCommand)), util.ColorInfo(u), pluginBinDir)

		// Look for other versions to cleanup
		files, err := ioutil.ReadDir(pluginBinDir)
		if err != nil {
			return path, err
		}
		deleted := make([]string, 0)
		// lets only delete plugins for this major version so we can keep, say, helm 2 and 3 around
		prefix := plugin.Name + "-"
		if len(version) > 0 {
			prefix += version[0:1]
		}
		for _, f := range files {
			if strings.HasPrefix(f.Name(), prefix) {
				err = os.Remove(filepath.Join(pluginBinDir, f.Name()))
				if err != nil {
					log.Logger().Warnf("Unable to delete old version of plugin %s installed at %s because %v", plugin.Name, f.Name(), err)
				} else {
					deleted = append(deleted, strings.TrimPrefix(f.Name(), fmt.Sprintf("%s-", plugin.Name)))
				}
			}
		}
		if len(deleted) > 0 {
			log.Logger().Infof("Deleted old plugin versions: %v", util.ColorInfo(deleted))
		}

		httpClient := util.GetClientWithTimeout(time.Minute * 20)

		// Get the file
		pluginURL, err := url.Parse(u)
		if err != nil {
			return "", err
		}
		filename := filepath.Base(pluginURL.Path)
		tmpDir, err := ioutil.TempDir("", plugin.Spec.Name)
		defer func() {
			err := os.RemoveAll(tmpDir)
			if err != nil {
				log.Logger().Errorf("Error cleaning up tmpdir %s because %v", tmpDir, err)
			}
		}()
		if err != nil {
			return "", err
		}
		downloadFile := filepath.Join(tmpDir, filename)
		// Create the file
		out, err := os.Create(downloadFile)
		if err != nil {
			return path, err
		}
		defer out.Close()
		requestU := u
		if pluginURL.User != nil {
			copy := *pluginURL
			copy.User = nil
			requestU = copy.String()
		}
		req, err := http.NewRequest("GET", requestU, nil)
		req.Header.Add("Accept", "application/octet-stream")
		if pluginURL.User != nil {
			pwd, ok := pluginURL.User.Password()
			if ok {
				req.Header.Add("Authorization", fmt.Sprintf("token %s", pwd))
			}
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			return path, err
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", fmt.Errorf("unable to install plugin %s because %s getting %s", plugin.Name, resp.Status, u)
		}
		defer resp.Body.Close()

		// Write the body to file
		_, err = io.Copy(out, resp.Body)
		if err != nil {
			return path, err
		}

		oldPath := downloadFile
		if strings.HasSuffix(filename, ".tar.gz") || strings.HasSuffix(aliasFileName, ".tar.gz") {
			err = util.UnTargz(downloadFile, tmpDir, make([]string, 0))
			if err != nil {
				return "", err
			}
			oldPath = filepath.Join(tmpDir, plugin.Spec.Name)
		}
		if strings.HasSuffix(filename, ".zip") || strings.HasSuffix(aliasFileName, ".zip") {
			err = util.Unzip(downloadFile, tmpDir)
			if err != nil {
				return "", err
			}
			oldPath = filepath.Join(tmpDir, plugin.Spec.Name)
		}

		err = util.CopyFile(oldPath, path)
		if err != nil {
			return "", err
		}
		// Make the file executable
		err = os.Chmod(path, 0755)
		if err != nil {
			return path, err
		}
	}
	return path, nil
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
