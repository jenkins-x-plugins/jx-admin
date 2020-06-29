package helmer

// FakeHelmer a fake helmer
type FakeHelmer struct {
	CWD   string
	Repos map[string]string
}

// NewFakeHelmer creates a
func NewFakeHelmer() Helmer {
	return &FakeHelmer{
		Repos: map[string]string{},
	}
}

func (f *FakeHelmer) SetCWD(dir string) {
	f.CWD = dir
}

func (f *FakeHelmer) HelmBinary() string {
	return "helm"
}

func (f *FakeHelmer) AddRepo(repo, repoURL, username, password string) error {
	f.Repos[repo] = repoURL
	return nil
}

func (f *FakeHelmer) RemoveRepo(repo string) error {
	delete(f.Repos, repo)
	return nil
}

func (f *FakeHelmer) ListRepos() (map[string]string, error) {
	return f.Repos, nil
}

func (f *FakeHelmer) UpdateRepo() error {
	return nil
}

func (f *FakeHelmer) IsRepoMissing(repoURL string) (bool, string, error) {
	for k, v := range f.Repos {
		if v == repoURL {
			return false, k, nil
		}
	}
	return true, "", nil
}

func (f *FakeHelmer) RemoveRequirementsLock() error {
	return nil
}

func (f *FakeHelmer) BuildDependency() error {
	return nil
}

func (f *FakeHelmer) InstallChart(chart string, releaseName string, ns string, version string, timeout int,
	values []string, valueStrings []string, valueFiles []string, repo string, username string, password string) error {
	return nil
}

func (f *FakeHelmer) UpgradeChart(chart string, releaseName string, ns string, version string, install bool, timeout int, force bool, wait bool,
	values []string, valueStrings []string, valueFiles []string, repo string, username string, password string) error {
	return nil
}

func (f *FakeHelmer) FetchChart(chart string, version string, untar bool, untardir string, repo string, username string,
	password string) error {
	return nil
}

func (f *FakeHelmer) DeleteRelease(ns string, releaseName string, purge bool) error {
	return nil
}

func (f *FakeHelmer) ListReleases(ns string) (map[string]ReleaseSummary, []string, error) {
	return nil, nil, nil
}

func (f *FakeHelmer) FindChart() (string, error) {
	return "", nil
}

func (f *FakeHelmer) PackageChart() error {
	return nil
}

func (f *FakeHelmer) StatusRelease(ns string, releaseName string) error {
	return nil
}

func (f *FakeHelmer) StatusReleaseWithOutput(ns string, releaseName string, format string) (string, error) {
	return "", nil
}

func (f *FakeHelmer) Lint(valuesFiles []string) (string, error) {
	return "", nil
}

func (f *FakeHelmer) Version(tls bool) (string, error) {
	return "", nil
}

func (f *FakeHelmer) SearchCharts(filter string, allVersions bool) ([]ChartSummary, error) {
	return nil, nil
}

func (f *FakeHelmer) Env() map[string]string {
	return nil
}

func (f *FakeHelmer) DecryptSecrets(location string) error {
	return nil
}

func (f *FakeHelmer) Template(chartDir string, releaseName string, ns string, outputDir string, upgrade bool, values []string, valueStrings []string, valueFiles []string) error {
	return nil
}
