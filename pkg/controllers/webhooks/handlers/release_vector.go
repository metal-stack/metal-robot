package handlers

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/blang/semver"
	v3 "github.com/google/go-github/v32/github"
	"github.com/metal-stack/metal-robot/pkg/controllers"
	"github.com/metal-stack/metal-robot/pkg/controllers/webhooks/handlers/actions"
	"github.com/metal-stack/metal-robot/pkg/controllers/webhooks/handlers/git"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	versionPrefix = "v"
)

type ReleaseVectorParams struct {
	Logger         *zap.SugaredLogger
	RepositoryName string
	TagName        string
	Client         *v3.Client
	AppClient      *v3.Client
	InstallID      int64
}

type ReleaseRepo struct {
	Name    string
	URL     string
	Patches actions.YAMLPathVersionPatches
}

var (
	releasePRBranch      = "develop"
	releaseCommitMessage = "Bump %s to version %s"
	releaseVectorRepos   = map[string]ReleaseRepo{
		"metal-api": {
			Name: "releases",
			URL:  "https://github.com/metal-stack/releases",
			Patches: actions.YAMLPathVersionPatches{
				{File: "release.yaml", YAMLPath: "docker-images.metal-stack.control-plane.metal-api.tag"}},
		},
		"masterdata-api": {
			Name: "releases",
			URL:  "https://github.com/metal-stack/releases",
			Patches: actions.YAMLPathVersionPatches{
				{File: "release.yaml", YAMLPath: "docker-images.metal-stack.control-plane.masterdata-api.tag"},
			},
		},
		"metal-console": {
			Name: "releases",
			URL:  "https://github.com/metal-stack/releases",
			Patches: actions.YAMLPathVersionPatches{
				{File: "release.yaml", YAMLPath: "docker-images.metal-stack.control-plane.metal-console.tag"},
			},
		},
		"metalctl": {
			Name: "releases",
			URL:  "https://github.com/metal-stack/releases",
			Patches: actions.YAMLPathVersionPatches{
				{File: "release.yaml", YAMLPath: "docker-images.metal-stack.control-plane.metalctl.tag"},
				{File: "release.yaml", YAMLPath: "binaries.metal-stack.metalctl.version"},
				{File: "release.yaml", YAMLPath: "binaries.metal-stack.metalctl.linux.url", Template: "https://github.com/metal-stack/metalctl/releases/download/%s/metalctl-linux-amd64"},
				{File: "release.yaml", YAMLPath: "binaries.metal-stack.metalctl.windows.url", Template: "https://github.com/metal-stack/metalctl/releases/download/%s/metalctl-windows-amd64"},
				{File: "release.yaml", YAMLPath: "binaries.metal-stack.metalctl.darwin.url", Template: "https://github.com/metal-stack/metalctl/releases/download/%s/metalctl-darwin-amd64"},
			},
		},
		"metal-core": {
			Name: "releases",
			URL:  "https://github.com/metal-stack/releases",
			Patches: actions.YAMLPathVersionPatches{
				{File: "release.yaml", YAMLPath: "docker-images.metal-stack.partition.metal-core.tag"},
			},
		},
		"ipmi-catcher": {
			Name: "releases",
			URL:  "https://github.com/metal-stack/releases",
			Patches: actions.YAMLPathVersionPatches{
				{File: "release.yaml", YAMLPath: "docker-images.metal-stack.control-plane.ipmi-catcher.tag"},
			},
		},

		"backup-restore-sidecar": {
			Name: "releases",
			URL:  "https://github.com/metal-stack/releases",
			Patches: actions.YAMLPathVersionPatches{
				{File: "release.yaml", YAMLPath: "docker-images.metal-stack.generic.backup-restore-sidecar.tag"},
			},
		},
		"metal-dockerfiles": {
			Name: "releases",
			URL:  "https://github.com/metal-stack/releases",
			Patches: actions.YAMLPathVersionPatches{
				{File: "release.yaml", YAMLPath: "docker-images.metal-stack.generic.deployment-base.tag"},
			},
		},

		"gardener-extension-provider-metal": {
			Name: "releases",
			URL:  "https://github.com/metal-stack/releases",
			Patches: actions.YAMLPathVersionPatches{
				{File: "release.yaml", YAMLPath: "docker-images.metal-stack.gardener.gardener-extension-provider-metal.tag"},
			},
		},
		"os-metal-extension": {
			Name: "releases",
			URL:  "https://github.com/metal-stack/releases",
			Patches: actions.YAMLPathVersionPatches{
				{File: "release.yaml", YAMLPath: "docker-images.metal-stack.gardener.os-metal-extension.tag"},
			},
		},
		"firewall-controller": {
			Name: "releases",
			URL:  "https://github.com/metal-stack/releases",
			Patches: actions.YAMLPathVersionPatches{
				{File: "release.yaml", YAMLPath: "docker-images.metal-stack.gardener.firewall-controller.tag"},
			},
		},

		"csi-lvm": {
			Name: "releases",
			URL:  "https://github.com/metal-stack/releases",
			Patches: actions.YAMLPathVersionPatches{
				{File: "release.yaml", YAMLPath: "docker-images.metal-stack.kubernetes.csi-lvm-controller.tag"},
				{File: "release.yaml", YAMLPath: "docker-images.metal-stack.kubernetes.csi-lvm-provisioner.tag"},
			},
		},
		"metal-ccm": {
			Name: "releases",
			URL:  "https://github.com/metal-stack/releases",
			Patches: actions.YAMLPathVersionPatches{
				{File: "release.yaml", YAMLPath: "docker-images.metal-stack.kubernetes.metal-ccm.tag"},
			},
		},
		"kubernetes-splunk-audit-webhook": {
			Name: "releases",
			URL:  "https://github.com/metal-stack/releases",
			Patches: actions.YAMLPathVersionPatches{
				{File: "release.yaml", YAMLPath: "docker-images.metal-stack.kubernetes.splunk-audit-webhook.tag"},
			},
		},

		"metal-roles": {
			Name: "releases",
			URL:  "https://github.com/metal-stack/releases",
			Patches: actions.YAMLPathVersionPatches{
				{File: "release.yaml", YAMLPath: "ansible-roles.metal-roles.version"},
			},
		},
		"ansible-common": {
			Name: "releases",
			URL:  "https://github.com/metal-stack/releases",
			Patches: actions.YAMLPathVersionPatches{
				{File: "release.yaml", YAMLPath: "ansible-roles.ansible-common.version"},
			},
		},
		// Just for testing
		// "metal-robot": {
		// 	Name: "metal-robot",
		// 	URL:  "https://github.com/metal-stack/metal-robot",
		// 	Patches: actions.YAMLPathVersionPatches{
		// 		{File: "deploy/kubernetes.yaml", YAMLPath: "metadata.name"},
		// 	},
		// },
	}
)

// AddToRelaseVector adds a release to the release vector in a release repository
func AddToRelaseVector(p *ReleaseVectorParams) error {
	releaseRepo, ok := releaseVectorRepos[p.RepositoryName]
	if !ok {
		p.Logger.Debugw("skip adding new version to release vector because not a release vector repo", "repo", p.RepositoryName, "release", p.TagName)
		return nil
	}
	tag := p.TagName
	if !strings.HasPrefix(tag, "v") {
		p.Logger.Debugw("skip adding new version to release vector because not starting with v", "repo", p.RepositoryName, "release", tag)
		return nil
	}

	version, err := semver.Make(tag[1:])
	if err != nil {
		return errors.Wrap(err, "not a valid semver release tag")
	}

	t, _, err := p.AppClient.Apps.CreateInstallationToken(context.Background(), p.InstallID, &v3.InstallationTokenOptions{})
	if err != nil {
		return errors.Wrap(err, "error creating installation token")
	}

	repoURL, err := url.Parse(releaseRepo.URL)
	if err != nil {
		return err
	}
	repoURL.User = url.UserPassword("x-access-token", t.GetToken())

	r, err := git.ShallowClone(repoURL.String(), releasePRBranch, 1)
	if err != nil {
		return err
	}

	reader := func(file string) ([]byte, error) {
		return git.ReadRepoFile(r, file)
	}

	writer := func(file string, content []byte) error {
		return git.WriteRepoFile(r, file, content)
	}

	err = releaseRepo.Patches.Apply(reader, writer, version, versionPrefix)
	if err != nil {
		return errors.Wrap(err, "error applying release updates")
	}

	commitMessage := fmt.Sprintf(releaseCommitMessage, p.RepositoryName, tag)
	hash, err := git.CommitAndPush(r, commitMessage)
	if err != nil {
		if err == git.NoChangesError {
			p.Logger.Debugw("skip adding new version to release vector because nothing changed", "repo", p.RepositoryName, "release", tag)
			return nil
		}
		return errors.Wrap(err, "error pushing release file")
	}

	p.Logger.Infow("pushed to release repo", "repo", releaseRepo.Name, "release", tag, "branch", releasePRBranch, "hash", hash)

	pr, _, err := p.Client.PullRequests.Create(context.Background(), controllers.GithubOrganisation, releaseRepo.Name, &v3.NewPullRequest{
		Title:               v3.String("Next release"),
		Head:                v3.String(releasePRBranch),
		Base:                v3.String("master"),
		Body:                v3.String("Next release of metal-stack"),
		MaintainerCanModify: v3.Bool(true),
	})
	if err != nil {
		if !strings.Contains(err.Error(), "A pull request already exists") {
			return err
		}
	} else {
		p.Logger.Infow("created pull request", "url", pr.GetURL())
	}

	return nil
}
