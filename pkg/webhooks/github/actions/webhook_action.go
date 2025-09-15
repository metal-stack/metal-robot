package actions

import "context"

type WebhookHandler[P any] interface {
	Handle(ctx context.Context, params P) error
}
