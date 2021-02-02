# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod download 
BINARY_NAME=vmwriter
BINARY_NAME_LIST=vmlist
BINARY_NAME_LIST_UNIX=$(BINARY_NAME_LIST)_bin
BINARY_UNIX=$(BINARY_NAME)_bin
BINARY_NAME_PR=promxycfg
BINARY_NAME_PR_UNIX=$(BINARY_NAME_PR)_bin

latest_tag=$$(git describe --abbrev=0 --tags)

all: test build
build: 
	$(GOBUILD) -o $(BINARY_NAME) cmd/vmwriter/main.go
	$(GOBUILD) -o $(BINARY_NAME_LIST) cmd/vmlist/main.go
	$(GOBUILD) -o $(BINARY_NAME_PR) cmd/promxycfg/main.go

test: 
	$(GOTEST) -v ./...
clean: 
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_UNIX)
	rm -r $(BINARY_NAME_LIST)
	rm -r $(BINARY_NAME_LIST_UNIX)
	rm -r $(BINARY_NAME_PR)
	rm -r $(BINARY_NAME_PR_UNIX)
deps:
	$(GOMOD) 


# Cross compilation
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BINARY_UNIX) -v cmd/vmwriter/main.go
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BINARY_NAME_LIST_UNIX) -v cmd/vmlist/main.go
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BINARY_NAME_PR_UNIX) -v cmd/promxycfg/main.go
