package controllers

import "time"

const (
	// GithubOrganisation is the name of the Github organisation where the metal-robot acts on
	GithubOrganisation = "metal-stack"
	// WebhookHandleTimeout is the duration after which a handler function context times out
	WebhookHandleTimeout = 30 * time.Second
)
