package actions_test

import (
	"context"
	"fmt"
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

				actions.Register("handler-a", &noopHandler{}, func(event *github.ReleaseEvent) (*noopHandlerParams, error) {
					return &noopHandlerParams{
						callbackFn: func() error {
							require.NotNil(t, event.Action)
							assert.Equal(t, "open", *event.Action)
							wg.Done()
							return nil
						},
					}, nil
				})

				actions.Register("handler-b", &noopHandler{}, func(event *github.RepositoryEvent) (*noopHandlerParams, error) {
					return &noopHandlerParams{
						callbackFn: func() error {
							assert.Fail(t, "this should not be called")
							return fmt.Errorf("shoulud not be called")
						},
					}, nil
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

type noopHandler struct{}

type noopHandlerParams struct {
	callbackFn func() error
}

func (*noopHandler) Handle(ctx context.Context, log *slog.Logger, params *noopHandlerParams) error {
	return params.callbackFn()
}
