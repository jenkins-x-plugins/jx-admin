package gitconfig

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/cli/cli/api"
	"github.com/cli/cli/auth"
	"github.com/jenkins-x/jx-helpers/pkg/termcolor"
	"github.com/jenkins-x/jx-logging/pkg/log"
	"gopkg.in/yaml.v3"
)

const (
	oauthHost = "github.com"
)

var (
	// The "JXL CLI" OAuth app
	oauthClientID = "0dae07a028587d292456"
	// This value is safe to be embedded in version control
	/* #nosec */
	oauthClientSecret = "3ee074dd53a806ff90865d0d2b4f30597429553e"
)

func setupConfigFile(filename string) (*ConfigEntry, error) {
	var verboseStream io.Writer
	if strings.Contains(os.Getenv("DEBUG"), "oauth") {
		verboseStream = os.Stderr
	}

	flow := &auth.OAuthFlow{
		Hostname:     oauthHost,
		ClientID:     oauthClientID,
		ClientSecret: oauthClientSecret,
		WriteSuccessHTML: func(w io.Writer) {
			// noinspection GoPrintFunctions
			fmt.Fprintln(w, oauthSuccessPage)
		},
		VerboseStream: verboseStream,
	}

	fmt.Fprintln(os.Stderr, "Notice: authentication required")
	fmt.Fprintf(os.Stderr, "Press Enter to open %s in your browser... ", flow.Hostname)
	err := waitForEnter(os.Stdin)
	if err != nil {
		return nil, err
	}
	token, err := flow.ObtainAccessToken()
	if err != nil {
		return nil, err
	}

	userLogin, err := getViewer(token)
	if err != nil {
		return nil, err
	}
	entry := ConfigEntry{
		User:  userLogin,
		Token: token,
	}
	data := make(map[string][]ConfigEntry)
	data[flow.Hostname] = []ConfigEntry{entry}

	err = os.MkdirAll(filepath.Dir(filename), 0771)
	if err != nil {
		return nil, err
	}

	config, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return nil, err
	}
	defer config.Close()

	yamlData, err := yaml.Marshal(data)
	if err != nil {
		return nil, err
	}
	n, err := config.Write(yamlData)
	if err == nil && n < len(yamlData) {
		err = io.ErrShortWrite
	}

	if err == nil {
		log.Logger().Infof(termcolor.ColorInfo("Please now grant access to any Organisations you require ") + ", this is needed to create git repositories used to manage environments (GitOps)")
		fmt.Fprintln(os.Stderr, termcolor.ColorInfo(fmt.Sprintf("https://github.com/settings/connections/applications/%s", oauthClientID)))
		fmt.Fprintln(os.Stderr, "Press Enter to continue... ")
		err = waitForEnter(os.Stdin)
		if err != nil {
			return &entry, err
		}

		fmt.Fprintln(os.Stderr, "Authentication complete. Press Enter to continue... ")
		err := waitForEnter(os.Stdin)
		if err != nil {
			return &entry, err
		}
	}
	return &entry, err
}

func getViewer(token string) (string, error) {
	http := api.NewClient(api.AddHeader("Authorization", fmt.Sprintf("token %s", token)))

	response := struct {
		Viewer struct {
			Login string
		}
	}{}
	err := http.GraphQL("{ viewer { login } }", nil, &response)
	return response.Viewer.Login, err
}

func waitForEnter(r io.Reader) error {
	scanner := bufio.NewScanner(r)
	scanner.Scan()
	return scanner.Err()
}
