package constants

import "time"

const (
	// WebhookHandleTimeout is the duration after which a webhook handle function context times out
	// the entire thing is asynchronuous anyway, so the VCS will get an immediate response, this is just
	// that we do not have processing of events hanging internally
	WebhookHandleTimeout = 120 * time.Second
)
