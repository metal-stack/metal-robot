package handlers_test

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"testing"

	"github.com/google/go-github/v79/github"
	"github.com/metal-stack/metal-lib/pkg/pointer"
	"github.com/metal-stack/metal-robot/pkg/webhooks/handlers"
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
				handlers.Run(log, &github.ReleaseEvent{
					Action: pointer.Pointer("open"),
				})
			},
		},
		{
			name: "different events",
			testFn: func(t *testing.T) {
				var wg sync.WaitGroup

				wg.Add(2)

				handlers.Register("handler-a", &noopHandler{}, func(event *github.ReleaseEvent) (*noopHandlerParams, error) {
					require.NotNil(t, event.Action)
					assert.Equal(t, "open", *event.Action)
					return &noopHandlerParams{
						callbackFn: func() error {
							wg.Done()
							return nil
						},
					}, nil
				})

				handlers.Register("handler-a", &noopHandler{}, func(event *github.ReleaseEvent) (*noopHandlerParams, error) {
					require.NotNil(t, event.Action)
					assert.Equal(t, "open", *event.Action)
					return &noopHandlerParams{
						callbackFn: func() error {
							wg.Done()
							return nil
						},
					}, nil
				})

				handlers.Register("handler-b", &noopHandler{}, func(event *github.RepositoryEvent) (*noopHandlerParams, error) {
					assert.Fail(t, "this should not be called")
					return &noopHandlerParams{
						callbackFn: func() error {
							t.Fail()
							return fmt.Errorf("shoulud not be called")
						},
					}, nil
				})

				handlers.Run(log, &github.ReleaseEvent{
					Action: pointer.Pointer("open"),
				})

				wg.Wait()
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer handlers.Clear()

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
