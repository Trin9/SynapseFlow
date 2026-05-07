.PHONY: apidoc apidoc-install

SWAG_BIN ?= $(shell go env GOPATH)/bin/swag
APIDOC_ENTRY ?= server.go
APIDOC_OUT ?= ./docs
APIDOC_DIRS ?= .,../mcp,../../pkg/models

apidoc-install:
	go install github.com/swaggo/swag/cmd/swag@latest

apidoc:
	@test -x "$(SWAG_BIN)" || $(MAKE) apidoc-install
	cd internal/api && "$(SWAG_BIN)" init -g "$(APIDOC_ENTRY)" -o "$(APIDOC_OUT)" --parseInternal --dir "$(APIDOC_DIRS)"
