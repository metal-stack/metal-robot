//go:build integration
// +build integration

package e2e

import (
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/go-github/v79/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testOrg = "metal-robot-test"
)

var (
	ghClient *github.Client = func() *github.Client {
		return github.NewClient(http.DefaultClient).WithAuthToken(os.Getenv("GITHUB_AUTH_TOKEN"))
	}()
)

func Test_E2E_Maintainers(t *testing.T) {
	cleanup := func() {
		resp, err := ghClient.Repositories.Delete(t.Context(), testOrg, "new-repo")
		if err != nil && resp.StatusCode != http.StatusNotFound {
			require.NoError(t, err)
		}

		resp, err = ghClient.Teams.DeleteTeamBySlug(t.Context(), testOrg, "new-repo-maintainers")
		if err != nil && resp.StatusCode != http.StatusNotFound {
			require.NoError(t, err)
		}
	}
	cleanup()
	defer cleanup()

	repo, _, err := ghClient.Repositories.Create(t.Context(), testOrg, &github.Repository{Name: github.Ptr("new-repo")})
	require.NoError(t, err)
	require.NotNil(t, repo)

	var (
		team *github.Team
	)
	require.Eventually(t, func() bool {
		team, _, err = ghClient.Teams.GetTeamBySlug(t.Context(), testOrg, "new-repo-maintainers")
		if err == nil {
			return true
		}

		return false
	}, 30*time.Second, 3*time.Second)

	assert.Equal(t, "Maintainers of new-repo", *team.Description)
	assert.Equal(t, 1, *team.MembersCount)
}
