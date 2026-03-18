.PHONY: build install test docker-xelatex

build:
	go build -o bin/mutercim ./cmd/mutercim

install:
	go install ./cmd/mutercim

test:
	go test ./...

docker-xelatex:
	docker build -t mutercim/xelatex docker/xelatex/
