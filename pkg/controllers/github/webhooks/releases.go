package webhooks

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/url"
	"strings"

	"github.com/blang/semver"
	v3 "github.com/google/go-github/v32/github"
	"github.com/metal-stack/metal-robot/pkg/controllers/utils"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/go-git/go-billy/v5/util"
	"gopkg.in/go-playground/webhooks.v5/github"
)

const (
	owner = "metal-stack"
)

type ReleaseProcessor struct {
	Logger    *zap.SugaredLogger
	Payload   *github.ReleasePayload
	Client    *v3.Client
	AppClient *v3.Client
	InstallID int64
}

type releaseUpdates []releaseUpdate
type releaseUpdate struct {
	YAMLPath string
	Template string
}

const (
	releasePRBranch = "develop"
	releaseRepoName = "releases"
	releaseRepoURL  = "https://github.com/metal-stack/releases"
	releaseFile     = "release.yaml"
	commitMessage   = "Bump %s to version %s"
)

var (
	releaseVectorRepos = map[string]releaseUpdates{
		"metal-api": {
			{
				YAMLPath: "docker-images.metal-stack.control-plane.metal-api.tag",
			},
		},
		"masterdata-api": {
			{
				YAMLPath: "docker-images.metal-stack.control-plane.masterdata-api.tag",
			},
		},
		"metal-console": {
			{
				YAMLPath: "docker-images.metal-stack.control-plane.metal-console.tag",
			},
		},
		"metalctl": {
			{
				YAMLPath: "docker-images.metal-stack.control-plane.metalctl.tag",
			},
			{
				YAMLPath: "binaries.metal-stack.metalctl.version",
			},
			{
				YAMLPath: "binaries.metal-stack.metalctl.linux.url",
				Template: "https://github.com/metal-stack/metalctl/releases/download/%s/metalctl-linux-amd64",
			},
			{
				YAMLPath: "binaries.metal-stack.metalctl.windows.url",
				Template: "https://github.com/metal-stack/metalctl/releases/download/%s/metalctl-windows-amd64",
			},
			{
				YAMLPath: "binaries.metal-stack.metalctl.darwin.url",
				Template: "https://github.com/metal-stack/metalctl/releases/download/%s/metalctl-darwin-amd64",
			},
		},

		"metal-core": {
			{
				YAMLPath: "docker-images.metal-stack.partition.metal-core.tag",
			},
		},
		"ipmi-catcher": {
			{
				YAMLPath: "docker-images.metal-stack.control-plane.ipmi-catcher.tag",
			},
		},

		"backup-restore-sidecar": {
			{
				YAMLPath: "docker-images.metal-stack.generic.backup-restore-sidecar.tag",
			},
		},
		"metal-dockerfiles": {
			{
				YAMLPath: "docker-images.metal-stack.generic.deployment-base.tag",
			},
		},

		"gardener-extension-provider-metal": {
			{
				YAMLPath: "docker-images.metal-stack.gardener.gardener-extension-provider-metal.tag",
			},
		},
		"os-metal-extension": {
			{
				YAMLPath: "docker-images.metal-stack.gardener.os-metal-extension.tag",
			},
		},
		"firewall-controller": {
			{
				YAMLPath: "docker-images.metal-stack.gardener.firewall-controller.tag",
			},
		},

		"csi-lvm": {
			{
				YAMLPath: "docker-images.metal-stack.kubernetes.csi-lvm-controller.tag",
			},
			{
				YAMLPath: "docker-images.metal-stack.kubernetes.csi-lvm-provisioner.tag",
			},
		},
		"metal-ccm": {
			{
				YAMLPath: "docker-images.metal-stack.kubernetes.metal-ccm.tag",
			},
		},
		"kubernetes-splunk-audit-webhook": {
			{
				YAMLPath: "docker-images.metal-stack.kubernetes.splunk-audit-webhook.tag",
			},
		},

		"metal-roles": {
			{
				YAMLPath: "ansible-roles.metal-roles.version",
			},
		},
		"ansible-common": {
			{
				YAMLPath: "ansible-roles.ansible-common.version",
			},
		},

		// just for testing:
		"metal-robot": {
			{
				YAMLPath: "docker-images.metal-stack.control-plane.metal-api.tag",
			},
			{
				YAMLPath: "binaries.metal-stack.metalctl.darwin.url",
				Template: "https://github.com/metal-stack/metalctl/releases/download/%s/metalctl-darwin-amd64",
			},
		},
	}
)

func ProcessReleaseEvent(p *ReleaseProcessor) error {
	return addToRelaseVector(p)
}

// addToRelaseVector adds a release to the release vector in a release repository
func addToRelaseVector(p *ReleaseProcessor) error {
	payload := p.Payload
	updates, ok := releaseVectorRepos[payload.Repository.Name]
	if !ok {
		p.Logger.Debugw("skip adding release to release vector because not a release vector repo", "repo", payload.Repository.Name, "release", payload.Release.TagName, "action", payload.Action)
		return nil
	}
	if payload.Action != "released" {
		p.Logger.Debugw("skip adding release to release vector because action was not released", "repo", payload.Repository.Name, "release", payload.Release.TagName, "action", payload.Action)
		return nil
	}
	tag := payload.Release.TagName
	if !strings.HasPrefix(tag, "v") {
		p.Logger.Debugw("skip adding release to release vector because not starting with v", "repo", payload.Repository.Name, "release", payload.Release.TagName, "action", payload.Action)
		return nil
	}

	version, err := semver.Make(tag[1:])
	if err != nil {
		return errors.Wrap(err, "not a valid semver release tag")
	}

	p.Logger.Infow("adding release to release vector", "repo", payload.Repository.Name, "release", tag)

	t, _, err := p.AppClient.Apps.CreateInstallationToken(context.Background(), p.InstallID, &v3.InstallationTokenOptions{})
	if err != nil {
		return errors.Wrap(err, "error creating installation token")
	}

	repoURL, err := url.Parse(releaseRepoURL)
	if err != nil {
		return err
	}
	repoURL.User = url.UserPassword("x-access-token", t.GetToken())

	r, err := utils.ShallowClone(repoURL.String(), releasePRBranch, 1)
	if err != nil {
		return err
	}

	w, err := r.Worktree()
	if err != nil {
		return errors.Wrap(err, "error retrieving git worktree")
	}

	f, err := w.Filesystem.Open(releaseFile)
	if err != nil {
		return errors.Wrap(err, "error opening release file")
	}
	defer f.Close()

	content, err := ioutil.ReadAll(f)
	if err != nil {
		return errors.Wrap(err, "error reading release file")
	}

	new, err := applyReleaseUpdates(content, updates, version)
	if err != nil {
		return errors.Wrap(err, "error applying release updates")
	}

	if string(new) == string(content) {
		p.Logger.Debugw("skip adding release to release vector because nothing changed", "repo", payload.Repository.Name, "release", payload.Release.TagName)
		return nil
	}

	err = util.WriteFile(w.Filesystem, releaseFile, new, 0755)
	if err != nil {
		return errors.Wrap(err, "error writing release file")
	}

	commitMessage := fmt.Sprintf(commitMessage, payload.Repository.Name, payload.Release.TagName)
	hash, err := utils.CommitAndPush(r, commitMessage)
	if err != nil {
		return err
	}

	p.Logger.Infow("pushed to release repo", "repo", releaseRepoURL, "branch", releasePRBranch, "hash", hash)

	pr, _, err := p.Client.PullRequests.Create(context.Background(), owner, releaseRepoName, &v3.NewPullRequest{
		Title:               v3.String("Next release"),
		Head:                v3.String(releasePRBranch),
		Base:                v3.String("master"),
		Body:                v3.String("Next release of metal-stack"),
		MaintainerCanModify: v3.Bool(true),
	})
	if err != nil {
		return errors.Wrap(err, "error creating pull request")
	}

	p.Logger.Infow("created pull request", "url", pr.GetURL())

	return nil
}

func applyReleaseUpdates(content []byte, updates releaseUpdates, version semver.Version) ([]byte, error) {
	new := content
	for _, update := range updates {
		old, err := utils.GetYAML(new, update.YAMLPath)
		if err != nil {
			return nil, errors.Wrap(err, "error retrieving path from release file")
		}

		value := "v" + version.String()

		if update.Template == "" {
			oldVersion, err := semver.Make(old[1:])
			if err != nil {
				return nil, err
			}

			if !version.GT(oldVersion) {
				continue
			}
		} else {
			value = fmt.Sprintf(update.Template, value)
		}

		new, err = utils.SetYAML(new, update.YAMLPath, value)
		if err != nil {
			return nil, err
		}
	}
	return new, nil
}
