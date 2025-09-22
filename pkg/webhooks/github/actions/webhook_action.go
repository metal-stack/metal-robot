package actions

import (
	"context"
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
		Handle(ctx context.Context, params P) error
	}

	WebhookEvents interface {
		*github.ReleaseEvent | *github.RepositoryEvent
	}

	handleFn[E WebhookEvents] func(ctx context.Context, event E) error

	key[T WebhookEvents] struct{}
)

func Run[E WebhookEvents](log *slog.Logger, e E) {
	mtx.RLock()
	defer mtx.RUnlock()

	if val, ok := handlerMap[key[E]{}]; ok {
		for _, h := range val {
			fn := h.(handleFn[E]) // not checked because can only be filled by Append

			go func() {
				// handlers can run in parallel, so create an own context for every handler
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
				defer cancel()

				if err := fn(ctx, e); err != nil {
					log.Error("error handling repository event", "error", err)
				}
			}()
		}
	}
}

func Append[E WebhookEvents](v handleFn[E]) {
	mtx.Lock()
	defer mtx.Unlock()

	handlerMap[key[E]{}] = append(handlerMap[key[E]{}], v)
}
