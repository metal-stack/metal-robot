package git

import (
	"io/ioutil"
	"time"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-billy/v5/util"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/pkg/errors"
)

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
		return nil, errors.Wrap(err, "error cloning git repo")
	}

	w, err := r.Worktree()
	if err != nil {
		return nil, errors.Wrap(err, "error retrieving git worktree")
	}

	err = r.Fetch(&git.FetchOptions{
		RefSpecs: []config.RefSpec{"refs/*:refs/*", "HEAD:refs/heads/HEAD"},
	})
	if err != nil {
		return nil, errors.Wrap(err, "error fetching repository refs")
	}

	err = w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.ReferenceName(defaultLocalRef + "/" + branch),
		Force:  true,
	})
	if err != nil {
		switch err {
		case plumbing.ErrReferenceNotFound:
			err2 := w.Checkout(&git.CheckoutOptions{
				Branch: plumbing.ReferenceName(defaultLocalRef + "/" + branch),
				Force:  true,
				Create: true,
			})
			if err2 != nil {
				return nil, errors.Wrap(err2, "error during git checkout")
			}
		default:
			return nil, errors.Wrap(err, "error during git checkout")
		}
	}

	return r, nil
}

func CommitAndPush(r *git.Repository, msg string) (string, error) {
	w, err := r.Worktree()
	if err != nil {
		return "", errors.Wrap(err, "error getting worktree")
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
		return "", errors.Wrap(err, "error during git commit")
	}

	branch, err := GetCurrentBranchFromRepository(r)
	if err != nil {
		return "", errors.Wrap(err, "error finding current branch")
	}

	err = r.Push(&git.PushOptions{
		RefSpecs: []config.RefSpec{
			config.RefSpec(branch + ":" + branch),
		},
	})
	if err != nil {
		return "", errors.Wrap(err, "error pushing to repo")
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
		return nil, errors.Wrap(err, "error retrieving git worktree")
	}

	f, err := w.Filesystem.Open(path)
	if err != nil {
		return nil, errors.Wrap(err, "error opening repository file")
	}
	defer f.Close()

	data, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, errors.Wrap(err, "error reading repository file")
	}

	return data, nil
}

func WriteRepoFile(r *git.Repository, path string, data []byte) error {
	w, err := r.Worktree()
	if err != nil {
		return errors.Wrap(err, "error retrieving git worktree")
	}

	f, err := w.Filesystem.Open(path)
	if err != nil {
		return errors.Wrap(err, "error opening repository file")
	}
	defer f.Close()

	err = util.WriteFile(w.Filesystem, path, data, 0755)
	if err != nil {
		return errors.Wrap(err, "error writing release file")
	}

	return nil
}
