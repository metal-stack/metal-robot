package git

import (
	"fmt"
	"io"
	"time"

	"errors"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-billy/v5/util"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
)

var NoChangesError = fmt.Errorf("no changes")

const (
	defaultLocalRef   = "refs/heads"
	defaultAuthor     = "metal-stack"
	defaultAuthorMail = "info@metal-stack.io"
)

func ShallowClone(url string, branch string, depth int) (*git.Repository, error) {
	r, err := git.Clone(memory.NewStorage(), memfs.New(), &git.CloneOptions{
		URL:   url,
		Depth: depth,
	})
	if err != nil {
		return nil, fmt.Errorf("error cloning git repo: %w", err)
	}

	w, err := r.Worktree()
	if err != nil {
		return nil, fmt.Errorf("error retrieving git worktree: %w", err)
	}

	err = r.Fetch(&git.FetchOptions{
		RefSpecs: []config.RefSpec{"refs/*:refs/*", "HEAD:refs/heads/HEAD"},
	})
	if err != nil {
		return nil, fmt.Errorf("error fetching repository refs: %w", err)
	}

	err = w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.ReferenceName(defaultLocalRef + "/" + branch),
		Force:  true,
	})
	if err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			err2 := w.Checkout(&git.CheckoutOptions{
				Branch: plumbing.ReferenceName(defaultLocalRef + "/" + branch),
				Force:  true,
				Create: true,
			})
			if err2 != nil {
				return nil, fmt.Errorf("error during git checkout: %w", err2)
			}
		} else {
			return nil, fmt.Errorf("error during git checkout: %w", err)
		}
	}

	return r, nil
}

func PushToRemote(remoteURL, remoteBranch, targetURL, targetBranch, msg string) error {
	r, err := git.Clone(memory.NewStorage(), memfs.New(), &git.CloneOptions{
		RemoteName:    "remote-repo",
		URL:           remoteURL,
		ReferenceName: plumbing.ReferenceName(defaultLocalRef + "/" + remoteBranch),
	})
	if err != nil {
		return fmt.Errorf("error cloning git repo: %w", err)
	}

	remote, err := r.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{targetURL},
	})
	if err != nil {
		return fmt.Errorf("error creating remote: %w", err)
	}

	err = remote.Push(&git.PushOptions{
		RemoteName: "origin",
		RefSpecs: []config.RefSpec{
			config.RefSpec(defaultLocalRef + "/" + remoteBranch + ":" + defaultLocalRef + "/" + targetBranch),
		},
		Force: true, // when the contributor does a force push, this will make it work anyway
	})
	if err != nil {
		return fmt.Errorf("error pushing to repo: %w", err)
	}

	return nil
}

func DeleteBranch(repoURL, branch string) error {
	r, err := git.Clone(memory.NewStorage(), memfs.New(), &git.CloneOptions{
		URL:   repoURL,
		Depth: 1,
	})
	if err != nil {
		return fmt.Errorf("error cloning git repo: %w", err)
	}

	err = r.Storer.RemoveReference(plumbing.NewBranchReferenceName(branch))
	if err != nil {
		return fmt.Errorf("error deleting branch in git repo: %w", err)
	}

	return nil
}

func CreateTag(repoURL, branch, tag, user string) error {
	r, err := git.Clone(memory.NewStorage(), memfs.New(), &git.CloneOptions{
		URL:   repoURL,
		Depth: 1,
	})
	if err != nil {
		return fmt.Errorf("error cloning git repo: %w", err)
	}

	w, err := r.Worktree()
	if err != nil {
		return fmt.Errorf("error retrieving git worktree: %w", err)
	}

	err = r.Fetch(&git.FetchOptions{
		RefSpecs: []config.RefSpec{"refs/*:refs/*", "HEAD:refs/heads/HEAD"},
	})
	if err != nil {
		return fmt.Errorf("error fetching repository refs: %w", err)
	}

	err = w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.ReferenceName(defaultLocalRef + "/" + branch),
		Force:  true,
	})
	if err != nil {
		return fmt.Errorf("error during git checkout: %w", err)
	}

	head, err := r.Head()
	if err != nil {
		return fmt.Errorf("error finding head: %w", err)
	}

	_, err = r.CreateTag(tag, head.Hash(), &git.CreateTagOptions{
		Tagger: &object.Signature{
			Name:  defaultAuthor,
			Email: defaultAuthorMail,
		},
		Message: "Bumped through metal-robot by " + user,
	})
	if err != nil {
		return fmt.Errorf("error creating tag: %w", err)
	}

	err = r.Push(&git.PushOptions{
		RemoteName: "origin",
		RefSpecs: []config.RefSpec{
			config.RefSpec("refs/tags/*:refs/tags/*"),
		},
	})
	if err != nil {
		return fmt.Errorf("error pushing to repo: %w", err)
	}

	return nil
}

func CommitAndPush(r *git.Repository, msg string) (string, error) {
	w, err := r.Worktree()
	if err != nil {
		return "", fmt.Errorf("error getting worktree: %w", err)
	}

	_, err = w.Add(".")
	if err != nil {
		return "", fmt.Errorf("error adding files to git index: %w", err)
	}

	status, err := w.Status()
	if err != nil {
		return "", fmt.Errorf("error getting git status: %w", err)
	}

	if status.IsClean() {
		return "", NoChangesError
	}

	hash, err := w.Commit(msg, &git.CommitOptions{
		Author: &object.Signature{
			Name:  defaultAuthor,
			Email: defaultAuthorMail,
			When:  time.Now(),
		},
		All: true,
	})
	if err != nil {
		return "", fmt.Errorf("error during git commit: %w", err)
	}

	branch, err := GetCurrentBranchFromRepository(r)
	if err != nil {
		return "", fmt.Errorf("error finding current branch: %w", err)
	}

	err = r.Push(&git.PushOptions{
		RefSpecs: []config.RefSpec{
			config.RefSpec(branch + ":" + branch),
		},
	})
	if err != nil {
		return "", fmt.Errorf("error pushing to repo: %w", err)
	}

	return hash.String(), nil
}

func GetCurrentBranchFromRepository(r *git.Repository) (string, error) {
	branchRefs, err := r.Branches()
	if err != nil {
		return "", err
	}

	headRef, err := r.Head()
	if err != nil {
		return "", err
	}

	var currentBranchName string
	err = branchRefs.ForEach(func(branchRef *plumbing.Reference) error {
		if branchRef.Hash() == headRef.Hash() {
			currentBranchName = branchRef.Name().String()

			return nil
		}

		return nil
	})
	if err != nil {
		return "", err
	}

	return currentBranchName, nil
}

func ReadRepoFile(r *git.Repository, path string) ([]byte, error) {
	w, err := r.Worktree()
	if err != nil {
		return nil, fmt.Errorf("error retrieving git worktree: %w", err)
	}

	f, err := w.Filesystem.Open(path)
	if err != nil {
		return nil, fmt.Errorf("error opening repository file: %w", err)
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("error reading repository file: %w", err)
	}

	return data, nil
}

func WriteRepoFile(r *git.Repository, path string, data []byte) error {
	w, err := r.Worktree()
	if err != nil {
		return fmt.Errorf("error retrieving git worktree: %w", err)
	}

	f, err := w.Filesystem.Open(path)
	if err != nil {
		return fmt.Errorf("error opening repository file: %w", err)
	}
	defer f.Close()

	err = util.WriteFile(w.Filesystem, path, data, 0755)
	if err != nil {
		return fmt.Errorf("error writing release file: %w", err)
	}

	return nil
}
