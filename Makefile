BINARY := metal-robot
MAINMODULE := github.com/metal-stack/metal-robot/cmd/metal-robot
COMMONDIR := $(or ${COMMONDIR},../builder)

include $(COMMONDIR)/Makefile.inc

.PHONY: all
all::
	go mod tidy

release:: all;
