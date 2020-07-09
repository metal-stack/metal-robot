package webhooks

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/url"
	"strings"
	"time"

	"github.com/blang/semver"
	v3 "github.com/google/go-github/v32/github"
	"github.com/metal-stack/metal-robot/pkg/controllers/utils"
	"go.uber.org/zap"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-billy/v5/util"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	"gopkg.in/go-playground/webhooks.v5/github"
)

type ReleaseProcessor struct {
	Logger    *zap.SugaredLogger
	Payload   *github.ReleasePayload
	Client    *v3.Client
	InstallID int64
}

type Repo struct {
	URL             string
	ReleaseJSONPath string
}

const (
	releasePRBranch = "develop"
	releaseRepo     = "https://github.com/metal-stack/releases"
	releaseFile     = "release.yaml"
	commitMessage   = "Bump %s to version %s"
)

var (
	releaseVectorRepos = map[string]Repo{
		"metal-api": {
			ReleaseJSONPath: "docker-images.metal-stack.control-plane.metal-api.tag",
		},
		// just for testing:
		// "metal-robot": {
		// 	ReleaseJSONPath: "docker-images.metal-stack.control-plane.metal-api.tag",
		// },
	}
)

func ProcessReleaseEvent(p *ReleaseProcessor) error {
	return addToRelaseVector(p)
}

// addToRelaseVector adds a release to the release vector in a release repository
func addToRelaseVector(p *ReleaseProcessor) error {
	payload := p.Payload
	repo, ok := releaseVectorRepos[payload.Repository.Name]
	if !ok {
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
		return err
	}

	p.Logger.Infow("adding release to release vector", "repo", payload.Repository.Name, "release", tag)
	t, _, err := p.Client.Apps.CreateInstallationToken(context.Background(), p.InstallID, &v3.InstallationTokenOptions{})
	if err != nil {
		return err
	}

	repoURL, err := url.Parse(releaseRepo)
	if err != nil {
		return err
	}

	repoURL.User = url.UserPassword("x-access-token", t.GetToken())
	r, err := git.Clone(memory.NewStorage(), memfs.New(), &git.CloneOptions{
		URL:   repoURL.String(),
		Depth: 1,
	})
	if err != nil {
		return err
	}

	w, err := r.Worktree()
	if err != nil {
		return err
	}

	err = w.Checkout(&git.CheckoutOptions{
		Branch: "refs/heads/" + releasePRBranch,
		Create: true,
		Force:  true,
	})
	if err != nil {
		return err
	}

	f, err := w.Filesystem.Open(releaseFile)
	if err != nil {
		return err
	}

	content, err := ioutil.ReadAll(f)
	if err != nil {
		return err
	}

	old, err := utils.GetYAML(content, repo.ReleaseJSONPath)
	if err != nil {
		return err
	}

	oldVersion, err := semver.Make(old[1:])
	if err != nil {
		return err
	}

	if !version.GT(oldVersion) {
		p.Logger.Debugw("skip adding release to release vector because not newer than current version", "repo", payload.Repository.Name, "release", payload.Release.TagName, "current", oldVersion.String(), "new", version.String())
		return nil
	}

	new, err := utils.SetYAML(content, repo.ReleaseJSONPath, payload.Release.TagName)
	if err != nil {
		return err
	}

	err = util.WriteFile(w.Filesystem, releaseFile, new, 0755)
	if err != nil {
		return err
	}

	hash, err := w.Commit(fmt.Sprintf(commitMessage, payload.Repository.Name, payload.Release.TagName), &git.CommitOptions{
		Author: &object.Signature{
			Name:  "metal-robot",
			Email: "info@metal-stac.io",
			When:  time.Now(),
		},
		All: true,
	})
	if err != nil {
		return err
	}

	c, _ := r.Config()
	p.Logger.Info(c.Remotes["origin"].URLs)
	p.Logger.Info(c.Branches)
	err = r.Push(&git.PushOptions{
		RefSpecs: []config.RefSpec{
			"refs/heads/" + releasePRBranch + ":" + "refs/heads/" + releasePRBranch,
		},
	})
	if err != nil {
		return err
	}

	p.Logger.Info("pushed to release repo", "repo", releaseRepo, "branch", releasePRBranch, "hash", hash.String())

	// TODO: compose release notes somehow?

	return nil
}

func generateSwaggerClients(payload *github.ReleasePayload) error {
	return nil
}

func prepareDraftReleaseNotes(payload *github.PushPayload) error {
	return nil
}
