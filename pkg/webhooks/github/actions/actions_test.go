package actions_test

import (
	"context"
	"log/slog"
	"sync"
	"testing"

	"github.com/google/go-github/v79/github"
	"github.com/metal-stack/metal-lib/pkg/pointer"
	"github.com/metal-stack/metal-robot/pkg/webhooks/github/actions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRun(t *testing.T) {
	log := slog.Default()

	tests := []struct {
		name   string
		testFn func(t *testing.T)
	}{
		{
			name: "no events",
			testFn: func(t *testing.T) {
				actions.Run(log, &github.ReleaseEvent{
					Action: pointer.Pointer("open"),
				})
			},
		},
		{
			name: "different events",
			testFn: func(t *testing.T) {
				var wg sync.WaitGroup

				wg.Add(1)

				actions.Append(func(_ context.Context, _ *slog.Logger, event *github.ReleaseEvent) error {
					require.NotNil(t, event.Action)
					assert.Equal(t, "open", *event.Action)
					wg.Done()
					return nil
				})

				actions.Append(func(_ context.Context, _ *slog.Logger, event *github.RepositoryEvent) error {
					assert.Fail(t, "this should not be called")
					return nil
				})

				actions.Run(log, &github.ReleaseEvent{
					Action: pointer.Pointer("open"),
				})

				wg.Wait()
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer actions.Clear()

			tt.testFn(t)
		})
	}
}
