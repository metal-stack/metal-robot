package constants

import "time"

const (
	// WebhookHandleTimeout is the duration after which a webhook handle function context times out
	WebhookHandleTimeout = 10 * time.Second
)
