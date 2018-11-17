PROJECT_ROOT := $(shell pwd)
VENDOR_PATH  := $(PROJECT_ROOT)/vendor
GO_FILES = $(shell find . -type f -name '*.go')
PACKAGES = $(shell ls src/)
GOPATH = $(shell pwd):$(shell pwd)/vendor:$(VENDOR_PATH)
BINARY = kc

all: build

build: $(GO_FILES)
	GOPATH=$(GOPATH) go build -o bin/$(BINARY)

test: $(GO_FILES)
	GOPATH=$(GOPATH) go test -v -cover $(PACKAGES) || exit 1;

run: 
	./bin/$(BINARY)

clean:
	rm -rf pkg/* bin/* vendor/*

package: build
	@cp -r bin package

lint:
	@GOPATH=$(VENDOR_PATH) go get -u github.com/golang/lint/golint
	$(VENDOR_PATH)/bin/golint src
syntax:
	go tool vet -v -all src/ || exit 1;

deps:
	@echo "Installing Dependencies..."
	@rm -rf $(VENDOR_PATH)
	@mkdir -p $(VENDOR_PATH) || exit 2
	@GOPATH=$(VENDOR_PATH) go get github.com/mitchellh/goamz/aws
	@GOPATH=$(VENDOR_PATH) go get github.com/segmentio/go-route53
	@GOPATH=$(VENDOR_PATH) go get k8s.io/client-go/...

