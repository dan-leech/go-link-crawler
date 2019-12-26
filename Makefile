GOCMD=GO111MODULE=on go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get

BINARY_NAME=go-link-crawler

# This version-strategy uses git tags to set the version string
VERSION := $(shell git describe --tags --always --dirty)
#
# This version-strategy uses a manual value to set the version string
#VERSION := 1.2.3

.PHONY: all # Test then build
all: build

.PHONY: build # Builds app for current architecture
build:
	$(GOBUILD) -o $(BINARY_NAME) -v main.go

.PHONY: clean # Cleans up build cache and executable
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	go clean --modcache

.PHONY: run # Runs app
run:
	$(GOBUILD) -o $(BINARY_NAME) -v main.go
	./$(BINARY_NAME)

.PHONY: run-docker # Runs app in docker container
run-docker:
	docker build -t $(BINARY_NAME) . -f Dockerfile
	docker run -it $(BINARY_NAME) ./main ./list.txt

.PHONY: version # Prints app version from git
version:
	@echo $(VERSION)

# List all commands
.PHONY: help # Generate list of targets with descriptions
help:
	@grep '^.PHONY: .* #' Makefile | sed 's/\.PHONY: \(.*\) # \(.*\)/\1 -> \2/' | expand -t20