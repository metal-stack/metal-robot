package handlers

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	glwebhooks "github.com/go-playground/webhooks/v6/gitlab"
	"github.com/google/go-github/v79/github"

	handlerrors "github.com/metal-stack/metal-robot/pkg/webhooks/handlers/errors"
)

type (
	// WebhookHandler is implemented by a handler to run a specific action.
	// It receives some arbitrary params, which are not specific to a certain webhook event.
	// This way you can run the same handler with different webhook events if desired.
	// Handlers can return a handlerrors.SkipErr error to indicate they did not need to run.
	WebhookHandler[Params any] interface {
		Handle(ctx context.Context, log *slog.Logger, params Params) error
	}

	// WebhookEvent describes an incoming event from a webhook.
	WebhookEvent interface {
		githubEvents | gitlabEvents
	}

	// ParamsConversion is a function that transforms a webhook event into parameters for a handler.
	// It can also contain some filter logic to skip the handler before calling it. For this
	// a handleerrors.SkipErr can be returned.
	ParamsConversion[Event WebhookEvent, Params any] func(event Event) (Params, error)

	key[Event WebhookEvent]   struct{}
	entry[Event WebhookEvent] struct {
		name   string
		invoke func(ctx context.Context, log *slog.Logger, event Event) error
	}

	// Now define explicit type constraints for events, so every new event needs to be whitelisted first.
	// This is basically just to prevent misuse of this package.

	githubEvents interface {
		*github.ReleaseEvent | *github.RepositoryEvent | *github.PullRequestEvent | *github.PushEvent |
			*github.ProjectV2ItemEvent | *github.IssueCommentEvent | *github.IssuesEvent
	}

	gitlabEvents interface {
		*glwebhooks.TagEventPayload
	}

	// eventTypeHandlers contains handlers by event type
	eventTypeHandlers = map[anyEventType][]anyHandler
	anyEventType      = any
	anyHandler        = any
)

var (
	// handlerMap contains a map of handlers grouped by their serve path, which then contains a list of handlers grouped by event type
	// => e.g. handlerMap["/webhook/path-a"][*github.ReleaseEvent][]{&handler.A{}, &handler.B{}}
	handlerMap = map[string]eventTypeHandlers{}
	mtx        sync.RWMutex
)

// Register registers a webhook handler by a given webhook event type. The conversion function transform the content of
// the webhook event into parameters for the handler and is called before the handler invocation.
// The name is only used for logging purposes and does not need to be identical with any contents from the application config.
func Register[Event WebhookEvent, Params any, Handler WebhookHandler[Params]](name string, path string, h Handler, convertFn ParamsConversion[Event, Params]) {
	mtx.Lock()
	defer mtx.Unlock()

	path = trimPath(path)

	handlers, ok := handlerMap[path]
	if !ok {
		handlers = eventTypeHandlers{}
	}

	handlers[key[Event]{}] = append(handlers[key[Event]{}], entry[Event]{
		name: name,
		invoke: func(ctx context.Context, log *slog.Logger, event Event) error {
			params, err := convertFn(event)
			if err != nil {
				return err
			}

			return h.Handle(ctx, log, params)
		},
	})

	handlerMap[path] = handlers
}

// Run triggers all registered handlers asynchronously for the given webhook event type.
func Run[Event WebhookEvent](log *slog.Logger, path string, e Event) {
	mtx.RLock()
	defer mtx.RUnlock()

	path = trimPath(path)

	eventHandlers, ok := handlerMap[path]
	if !ok {
		return
	}

	if val, ok := eventHandlers[key[Event]{}]; ok {
		for _, h := range val {
			var (
				data       = h.(entry[Event])
				handlerLog = log.With("handler-name", data.name)
			)

			go func() {
				// handlers can run in parallel, so create an own context for every handler
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
				defer cancel()

				err := data.invoke(ctx, handlerLog, e)
				if err != nil {
					var skipErr handlerrors.SkipErr
					if errors.As(err, &skipErr) {
						handlerLog.Debug("skip handling event", "reason", err.Error())
					} else {
						handlerLog.Error("error handling event", "error", err)
					}
				} else {
					handlerLog.Info("successfully handled event")
				}
			}()
		}
	}
}

// Clear can be used to clear all registered handlers. Basically this is only used for testing purposes.
func Clear() {
	mtx.Lock()
	defer mtx.Unlock()

	handlerMap = map[string]eventTypeHandlers{}
}

func trimPath(path string) string {
	return strings.Trim(path, "/")
}
