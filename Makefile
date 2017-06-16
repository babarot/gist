TEST?= $(shell go list ./... | grep -v vendor)
DEPS = $(shell go list -f '{{range .TestImports}}{{.}} {{end}}' ./...)
BIN  = gist
DIST = dist
VERSION   = $(shell grep 'Version =' cmd/root.go | sed -E 's/.*"(.+)"$$/\1/')
GOVERSION = $(shell go version)
GOOS    = $(word 1,$(subst /, ,$(lastword $(GOVERSION))))
GOARCH  = $(word 2,$(subst /, ,$(lastword $(GOVERSION))))
ARCNAME = $(BIN)-$(VERSION)-$(GOOS)-$(GOARCH)
RELDIR  = $(BIN)-$(GOOS)-$(GOARCH)

all: build

build: deps
	mkdir -p bin
	go build -o bin/$(BIN)

install: build
	go install
	if echo $$SHELL | grep "zsh" &>/dev/null; then \
		install -m 644 ./misc/completion/zsh/_gist $(shell zsh -c 'echo $$fpath[1]'); \
		fi

deps:
	go get -d -v ./...
	echo $(DEPS) | xargs -n1 go get -d

test: deps
	go test $(TEST) $(TESTARGS) -timeout=3s -parallel=4
	go vet $(TEST)
	go test $(TEST) -race

release:
	rm -rf $(DIST)/$(RELDIR)
	mkdir -p $(DIST)/$(RELDIR)
	go clean
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags='-X main.version=$(VERSION)' -o bin/$(BIN)$(SUFFIX_EXE)
	mv bin/$(BIN)$(SUFFIX_EXE) $(DIST)/$(RELDIR)/
	cp README.md $(DIST)/$(RELDIR)/
	cp ./misc/completion/zsh/_gist $(DIST)/$(RELDIR)/
	tar czf $(DIST)/$(ARCNAME).tar.gz -C $(DIST) $(RELDIR)
	rm -rf $(DIST)/$(RELDIR)/
	go clean

cross:
	@$(MAKE) release GOOS=windows GOARCH=amd64 SUFFIX_EXE=.exe
	@$(MAKE) release GOOS=windows GOARCH=386   SUFFIX_EXE=.exe
	@$(MAKE) release GOOS=linux   GOARCH=amd64
	@$(MAKE) release GOOS=linux   GOARCH=386
	@$(MAKE) release GOOS=darwin  GOARCH=amd64
	@$(MAKE) release GOOS=darwin  GOARCH=386

version:
	@echo $(VERSION)

.PHONY: all build deps test release cross version
