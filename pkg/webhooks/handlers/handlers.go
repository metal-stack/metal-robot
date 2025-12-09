package handlers

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	glwebhooks "github.com/go-playground/webhooks/v6/gitlab"
	"github.com/google/go-github/v79/github"

	handlerrors "github.com/metal-stack/metal-robot/pkg/webhooks/handlers/errors"
)

var (
	handlerMap = map[any][]any{} // unfortunately, this cannot be typed or I am just too stupid
	mtx        sync.RWMutex
)

type (
	WebhookHandler[P any] interface {
		Handle(ctx context.Context, log *slog.Logger, params P) error
	}

	WebhookEvents interface {
		githubEvents | gitlabEvents
	}

	githubEvents interface {
		*github.ReleaseEvent | *github.RepositoryEvent | *github.PullRequestEvent | *github.PushEvent |
			*github.ProjectV2ItemEvent | *github.IssueCommentEvent | *github.IssuesEvent
	}

	gitlabEvents interface {
		*glwebhooks.TagEventPayload
	}

	HandlerParamsFn[E WebhookEvents, P any] func(event E) (P, error)

	key[E WebhookEvents]   struct{}
	entry[E WebhookEvents] struct {
		name   string
		invoke func(ctx context.Context, log *slog.Logger, event E) error
	}
)

func Run[E WebhookEvents](log *slog.Logger, e E) {
	mtx.RLock()
	defer mtx.RUnlock()

	if val, ok := handlerMap[key[E]{}]; ok {
		for _, h := range val {
			data := h.(entry[E])

			log = log.With("handler-name", data.name)

			go func() {
				// handlers can run in parallel, so create an own context for every handler
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
				defer cancel()

				err := data.invoke(ctx, log, e)
				if err != nil {
					var skipErr handlerrors.SkipErr
					if errors.As(err, &skipErr) {
						log.Debug("skip handling event", "reason", err.Error())
					} else {
						log.Error("error handling event", "error", err)
					}
				} else {
					log.Info("successfully handled event")
				}
			}()
		}
	}
}

func Register[E WebhookEvents, P any, H WebhookHandler[P]](name string, h H, paramsFn HandlerParamsFn[E, P]) {
	mtx.Lock()
	defer mtx.Unlock()

	handlerMap[key[E]{}] = append(handlerMap[key[E]{}], entry[E]{
		name: name,
		invoke: func(ctx context.Context, log *slog.Logger, event E) error {
			params, err := paramsFn(event)
			if err != nil {
				return err
			}

			return h.Handle(ctx, log, params)
		},
	})
}

func Clear() {
	mtx.Lock()
	defer mtx.Unlock()

	handlerMap = map[any][]any{}
}
