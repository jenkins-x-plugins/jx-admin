package invitations

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/jenkins-x/jx-helpers/pkg/gitclient/giturl"

	"github.com/jenkins-x/jx-helpers/pkg/input/survey"

	"github.com/jenkins-x/jx-helpers/pkg/input"

	"github.com/jenkins-x/go-scm/scm"

	"github.com/jenkins-x/jx-helpers/pkg/gitclient/loadcreds"

	"github.com/jenkins-x/jx-logging/pkg/log"

	"github.com/pkg/errors"

	"github.com/jenkins-x/jx-helpers/pkg/scmhelpers"

	"github.com/jenkins-x/jx-admin/pkg/rootcmd"
	"github.com/jenkins-x/jx-helpers/pkg/cobras/helper"

	"github.com/jenkins-x/jx-helpers/pkg/cobras/templates"
	"github.com/spf13/cobra"
)

var (
	invitationsLong = templates.LongDesc(`
		List and accept git repository invitations for the pipeline bot user 
`)

	invitationsExample = templates.Examples(`
		# List and accept git repository invitations for the pipeline bot user
		%s invitations
	`)
)

// Options the options for creating a repository
type Options struct {
	Cmd    *cobra.Command
	client *scm.Client
	ctx    context.Context
	Args   []string
	Input  input.Interface

	OriginalRepos map[string]string
}

// NewCmdInvitations list and accept bot user invitations
func NewCmdInvitations() (*cobra.Command, *Options) {
	o := &Options{}

	cmd := &cobra.Command{
		Use:     "invitation",
		Aliases: []string{"invitations"},
		Short:   "Accept bot user invitations",
		Long:    invitationsLong,
		Example: fmt.Sprintf(invitationsExample, rootcmd.BinaryName),
		Run: func(cmd *cobra.Command, args []string) {
			o.Cmd = cmd
			o.Args = args
			err := o.Run()
			helper.CheckErr(err)
		},
	}
	o.Cmd = cmd

	return cmd, o
}

// Run implements the command
func (o *Options) Run() error {

	o.ctx = context.Background()
	botCredentials, err := loadcreds.FindOperatorCredentials()
	if err != nil {
		return errors.Wrapf(err, "failed to find jx operator credentials")
	}

	if o.Input == nil {
		o.Input = survey.NewInput()
	}
	// authenticate with SCM provider using the bot credentials
	gitServerURL := fmt.Sprintf("%s://%s", botCredentials.Protocol, botCredentials.Host)
	kind := giturl.SaasGitKind(gitServerURL)

	o.client, _, err = scmhelpers.NewScmClient(kind, gitServerURL, botCredentials.Password)
	if err != nil {
		return errors.Wrapf(err, "failed to create scm client")
	}

	// initially find any repository level invitations
	repoInvites, _, err := o.client.Users.ListInvitations(o.ctx)
	if err != nil {
		return errors.Wrapf(err, "failed to list users invitations")
	}

	// next find any organisation level invitations
	log.Logger().Infof("checking both repository and organization invitations for bot user %s", botCredentials.Username)
	memberships, _, err := o.client.Organizations.ListMemberships(o.ctx, scm.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to list team memberships")
	}
	orgInvites := []*scm.Membership{}
	for _, m := range memberships {
		if m.State == "pending" {
			orgInvites = append(orgInvites, m)
		}
	}

	log.Logger().Infof("found %d pending repository invites found for bot user %s", len(repoInvites), botCredentials.Username)
	log.Logger().Infof("found %d pending organization invites found for bot user %s", len(orgInvites), botCredentials.Username)

	// lets display the list of repository and organisation invites and ask user to select ones to accept
	var repoNamesToCheck []string
	for i, repo := range repoInvites {
		o.OriginalRepos[repo.Repo.Name] = repo.Repo.ID
		repoNamesToCheck = append(repoNamesToCheck, fmt.Sprintf("%d. Repository invite: %s/%s", i, gitServerURL, repo.Repo.Name))
	}

	// repo invites
	acceptRepoInvites, err := o.pickInvitesToAccept(repoNamesToCheck, repository)
	if err != nil {
		return errors.Wrapf(err, "failed to accept repository invites")
	}

	var orgNamesToCheck []string
	for i, org := range orgInvites {
		orgNamesToCheck = append(orgNamesToCheck, fmt.Sprintf("%d. Organization invite: %s/%s", i+1, gitServerURL, org.OrganizationName))
	}

	// org invites
	acceptOrgInvites, err := o.pickInvitesToAccept(orgNamesToCheck, organisation)
	if err != nil {
		return errors.Wrapf(err, "failed to accept organisation invites")
	}

	// accept the selected repo invites
	err = o.acceptRepoInvites(acceptRepoInvites)
	if err != nil {
		return errors.Wrapf(err, "failed to accept repository invites")
	}

	// accept the selected org invites
	err = o.acceptOrgInvites(acceptOrgInvites)
	if err != nil {
		return errors.Wrapf(err, "failed to accept repository invites")
	}
	return nil
}

type inviteType string

const (
	repository   inviteType = "repository"
	organisation inviteType = "organisation"
)

func (o *Options) pickInvitesToAccept(namesToCheck []string, kind inviteType) ([]string, error) {

	if len(namesToCheck) > 0 {
		acceptOrgNames, err := o.Input.SelectNames(namesToCheck, "Select invites to accept", false, "")
		if err != nil {
			return nil, errors.Wrapf(err, "error selecting invites to accept")
		}

		if acceptOrgNames == nil {
			log.Logger().Infof("no %s invites selected to accept", kind)
			return nil, nil
		}
		return acceptOrgNames, nil
	}
	return nil, nil
}

func (o *Options) acceptRepoInvites(invites []string) error {
	if invites == nil {
		return nil
	}
	reposToAccept, err := o.getInviteDetailsToAccept(invites, repository)
	if err != nil {
		return errors.Wrap(err, "failed to get repositories to accept")
	}
	for _, r := range reposToAccept {
		id, err := strconv.ParseInt(o.OriginalRepos[r], 10, 64)
		if err != nil {
			return errors.Wrapf(err, "failed to convert %s to an int64", o.OriginalRepos[r])
		}
		_, err = o.client.Users.AcceptInvitation(o.ctx, id)
		if err != nil {
			return errors.Wrapf(err, "failed to accept invite for repository %s", r)
		}
	}
	log.Logger().Infof("accepted invites to repositories %v", reposToAccept)
	return nil
}

func (o *Options) acceptOrgInvites(invites []string) error {
	if invites == nil {
		return nil
	}
	orgs, err := o.getInviteDetailsToAccept(invites, organisation)
	if err != nil {
		return errors.Wrap(err, "failed to get organisations to accept")
	}
	log.Logger().Infof("gonna accept orgs %v", orgs)
	for _, org := range orgs {
		_, err = o.client.Organizations.AcceptOrganizationInvitation(o.ctx, org)
		if err != nil {
			return errors.Wrapf(err, "failed to accept invite for organisation %s", org)
		}
	}
	log.Logger().Infof("accept invites to organisations %v", orgs)
	return nil
}

// extract a list of orgs or repos that a user has chosen to accept an invite for
func (o *Options) getInviteDetailsToAccept(invites []string, kind inviteType) ([]string, error) {
	var result []string
	for _, invite := range invites {
		// extract the repo or org name from the list of names we used in the survey to ask which repo invites should be accepted
		s := strings.SplitAfterN(invite, ":", 2)
		part2 := strings.TrimSpace(s[1])
		switch kind {
		case repository:
			gitInfo, err := giturl.ParseGitURL(part2)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse git url %s", part2)
			}
			result = append(result, fmt.Sprintf("%s/%s", gitInfo.Organisation, gitInfo.Name))
		case organisation:
			gitInfo, err := giturl.ParseGitOrganizationURL(part2)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse git url %s", part2)
			}
			if gitInfo.Organisation == "" {
				return nil, fmt.Errorf("failed to get git organisation from %s", part2)
			}
			result = append(result, gitInfo.Organisation)
		}
	}
	return result, nil
}
