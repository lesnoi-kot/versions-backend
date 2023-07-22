.PHONY: test

APP_PACKAGES = $(shell go list ./...)

test:
	go test $(APP_PACKAGES)
