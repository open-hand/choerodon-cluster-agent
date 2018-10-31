DOCKER_REGISTRY   ?=
IMAGE_PREFIX      ?=
SHORT_NAME        ?= choerodon-cluster-agent

# go option
GO        ?= go
LDFLAGS   :=
BINDIR    := $(CURDIR)/bin
TAGS      :=

.PHONY: all
all: build

.PHONY: build
build:
	GOBIN=$(BINDIR) $(GO) install -tags '$(TAGS)' -ldflags '$(LDFLAGS)' github.com/choerodon/choerodon-cluster-agent/...

.PHONY: clean
clean:
	rm -f bin/*

HAS_DEP := $(shell command -v dep;)
HAS_GIT := $(shell command -v git;)

.PHONY: bootstrap
bootstrap:
ifndef HAS_DEP
	$(GO) get -u github.com/golang/dep/cmd/dep
endif
ifndef HAS_GIT
	$(error You must install git)
endif
	dep ensure -v

.PHONY: test
test:
	$(GO) test ./...

.PHONY: coverage
coverage:
	$(GO) test -coverprofile=c.out ./...
	$(GO) tool cover -html=c.out

.PHONY: fmt
fmt:
	$(GO) fmt

include versioning.mk
