TEST?=./...
DEPS = $(shell go list -f '{{range .TestImports}}{{.}} {{end}}' ./...)

all: build

build: deps
	mkdir -p bin
	go build -o bin/gist

install: build
	install -m 755 ./bin/gist ~/bin/gist

deps:
	go get -d -v ./...
	echo $(DEPS) | xargs -n1 go get -d

test: deps
	go test $(TEST) $(TESTARGS) -timeout=3s -parallel=4
	go vet $(TEST)
	go test $(TEST) -race

.PHONY: all build deps test
