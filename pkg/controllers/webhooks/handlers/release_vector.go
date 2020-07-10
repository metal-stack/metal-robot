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

type ReleaseVectorParams struct {
	Logger         *zap.SugaredLogger
	RepositoryName string
	TagName        string
	Client         *v3.Client
	AppClient      *v3.Client
	InstallID      int64
}

const (
	releasePRBranch = "develop"
	releaseRepoName = "releases"
	releaseRepoURL  = "https://github.com/metal-stack/releases"
	releaseFile     = "release.yaml"
	commitMessage   = "Bump %s to version %s"
)

var (
	releaseVectorRepos = map[string][]actions.YAMLPathPatch{
		"metal-api": {
			{Path: "docker-images.metal-stack.control-plane.metal-api.tag"},
		},
		"masterdata-api": {
			{Path: "docker-images.metal-stack.control-plane.masterdata-api.tag"},
		},
		"metal-console": {
			{Path: "docker-images.metal-stack.control-plane.metal-console.tag"},
		},
		"metalctl": {
			{Path: "docker-images.metal-stack.control-plane.metalctl.tag"},
			{Path: "binaries.metal-stack.metalctl.version"},
			{Path: "binaries.metal-stack.metalctl.linux.url", Template: "https://github.com/metal-stack/metalctl/releases/download/%s/metalctl-linux-amd64"},
			{Path: "binaries.metal-stack.metalctl.windows.url", Template: "https://github.com/metal-stack/metalctl/releases/download/%s/metalctl-windows-amd64"},
			{Path: "binaries.metal-stack.metalctl.darwin.url", Template: "https://github.com/metal-stack/metalctl/releases/download/%s/metalctl-darwin-amd64"},
		},

		"metal-core": {
			{Path: "docker-images.metal-stack.partition.metal-core.tag"},
		},
		"ipmi-catcher": {
			{Path: "docker-images.metal-stack.control-plane.ipmi-catcher.tag"},
		},

		"backup-restore-sidecar": {
			{Path: "docker-images.metal-stack.generic.backup-restore-sidecar.tag"},
		},
		"metal-dockerfiles": {
			{Path: "docker-images.metal-stack.generic.deployment-base.tag"},
		},

		"gardener-extension-provider-metal": {
			{Path: "docker-images.metal-stack.gardener.gardener-extension-provider-metal.tag"},
		},
		"os-metal-extension": {
			{Path: "docker-images.metal-stack.gardener.os-metal-extension.tag"},
		},
		"firewall-controller": {
			{Path: "docker-images.metal-stack.gardener.firewall-controller.tag"},
		},

		"csi-lvm": {
			{Path: "docker-images.metal-stack.kubernetes.csi-lvm-controller.tag"},
			{Path: "docker-images.metal-stack.kubernetes.csi-lvm-provisioner.tag"},
		},
		"metal-ccm": {
			{Path: "docker-images.metal-stack.kubernetes.metal-ccm.tag"},
		},
		"kubernetes-splunk-audit-webhook": {
			{Path: "docker-images.metal-stack.kubernetes.splunk-audit-webhook.tag"},
		},

		"metal-roles": {
			{Path: "ansible-roles.metal-roles.version"},
		},
		"ansible-common": {
			{Path: "ansible-roles.ansible-common.version"},
		},
	}
)

// AddToRelaseVector adds a release to the release vector in a release repository
func AddToRelaseVector(p *ReleaseVectorParams) error {
	patches, ok := releaseVectorRepos[p.RepositoryName]
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

	repoURL, err := url.Parse(releaseRepoURL)
	if err != nil {
		return err
	}
	repoURL.User = url.UserPassword("x-access-token", t.GetToken())

	r, err := git.ShallowClone(repoURL.String(), releasePRBranch, 1)
	if err != nil {
		return err
	}

	content, err := git.ReadRepoFile(r, releaseFile)
	if err != nil {
		return err
	}

	patcher := actions.YAMLVersionPatcher{
		Patches: patches,
		Version: version,
		Content: content,
	}
	new, err := patcher.Patch()
	if err != nil {
		return errors.Wrap(err, "error applying release updates")
	}

	if string(new) == string(content) {
		p.Logger.Debugw("skip new version to release vector because nothing changed", "repo", p.RepositoryName, "release", tag)
		return nil
	}

	err = git.WriteRepoFile(r, releaseFile, new)
	if err != nil {
		return errors.Wrap(err, "error writing release file")
	}

	p.Logger.Infow("adding new version to release vector", "repo", p.RepositoryName, "release", tag)

	commitMessage := fmt.Sprintf(commitMessage, p.RepositoryName, tag)
	hash, err := git.CommitAndPush(r, commitMessage)
	if err != nil {
		return errors.Wrap(err, "error pushing release file")
	}

	p.Logger.Infow("pushed to release repo", "repo", releaseRepoURL, "branch", releasePRBranch, "hash", hash)

	pr, _, err := p.Client.PullRequests.Create(context.Background(), controllers.GithubOrganisation, releaseRepoName, &v3.NewPullRequest{
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
