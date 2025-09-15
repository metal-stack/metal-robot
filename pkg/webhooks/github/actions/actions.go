package actions

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/go-github/v74/github"
)

var (
	handlerMap = map[any][]any{}
	mtx        sync.RWMutex
)

type (
	WebhookHandler[P any] interface {
		Handle(ctx context.Context, log *slog.Logger, params P) error
	}

	WebhookEvents interface {
		*github.ReleaseEvent | *github.RepositoryEvent | *github.PullRequestEvent | *github.PushEvent |
			*github.ProjectV2ItemEvent | *github.IssueCommentEvent | *github.IssuesEvent
	}

	HandleFn[E WebhookEvents] func(ctx context.Context, log *slog.Logger, event E) error

	key[T WebhookEvents] struct{}
)

func Run[E WebhookEvents](log *slog.Logger, e E) {
	mtx.RLock()
	defer mtx.RUnlock()

	if val, ok := handlerMap[key[E]{}]; ok {
		for _, h := range val {
			var (
				handlerType = fmt.Sprintf("%T", h)
				log         = log.With("handler-type", handlerType)
				fn, ok      = h.(HandleFn[E])
			)

			if !ok {
				// this should never happen because handlers can only be added through Append()
				log.Error("handler map contains something different than a handler func")
				continue
			}

			go func() {
				// handlers can run in parallel, so create an own context for every handler
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
				defer cancel()

				log.Info("running event handler")

				if err := fn(ctx, log, e); err != nil {
					log.Error("error handling event", "error", err)
				} else {
					log.Info("successfully handled event")
				}
			}()
		}
	}
}

func Append[E WebhookEvents](v HandleFn[E]) {
	mtx.Lock()
	defer mtx.Unlock()

	handlerMap[key[E]{}] = append(handlerMap[key[E]{}], v)
}

func Clear() {
	mtx.Lock()
	defer mtx.Unlock()

	handlerMap = map[any][]any{}
}
