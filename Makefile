ifeq ($(CN), 1)
ENV := GOPROXY=https://goproxy.cn,direct
endif

SOURCE_FILES = $(shell go list -f '{{range .GoFiles}}{{$$.Dir}}/{{.}}\
{{end}}' ./...)

NS := github.com/projecteru2/vmihub
REVISION := $(shell git rev-parse HEAD || unknown)
BUILTAT := $(shell date +%Y-%m-%dT%H:%M:%S)
VERSION := $(shell git describe --tags $(shell git rev-list --tags --max-count=1))
GO_LDFLAGS ?= -X $(NS)/internal/version.REVISION=$(REVISION) \
              -X $(NS)/internal/version.BUILTAT=$(BUILTAT) \
              -X $(NS)/internal/version.VERSION=$(VERSION)
ifneq ($(KEEP_SYMBOL), 1)
	GO_LDFLAGS += -s
endif

BUILD := go build -race
TEST := go test -count=1 -race -cover -gcflags=all=-l

PKGS := $$(go list ./... | grep -v -P '$(NS)/3rd|vendor/|mocks|e2e|fs|webconsole|ovn')

.PHONY: all test e2e

default: build

build: bin/vmihub

bin/vmihub: $(SOURCE_FILES)
	$(BUILD) -ldflags '$(GO_LDFLAGS)' -o "$@" ./cmd/vmihub

lint: 
	golangci-lint run

format: vet
	gofmt -s -w $$(find . -iname '*.go' | grep -v -P '\./3rd|\./vendor/|mocks')

vet:
	go vet $(PKGS)

deps:
	$(ENV) go mod tidy
	$(ENV) go mod vendor

mock: deps
	mockery --dir internal/storage --output internal/storage/mocks --name Storage
	mockery --dir client/image --output client/image/mocks --all

clean:
	rm -fr bin/*

setup: setup-lint
	$(ENV) go install github.com/vektra/mockery/v2@latest
	$(ENV) go install github.com/swaggo/swag/cmd/swag@latest
	$(ENV) go install -tags 'mysql' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

setup-lint:
	$(ENV) go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.57.1

swag:
	swag init -g cmd/vmihub/main.go -o cmd/vmihub/docs

test:
ifdef RUN
	$(TEST) -v -run='${RUN}' $(PKGS)
else
	$(TEST) $(PKGS)
endif

e2e:
ifdef DIR
	cp -f e2e/config.toml e2e/${DIR}/config.toml
	cd e2e/${DIR} && ginkgo -r -p -- --config=`pwd`/config.toml
else
	cd e2e && ginkgo -r -p -- --config=`pwd`/config.toml
endif

db-migrate-setup:
	curl -L https://github.com/golang-migrate/migrate/releases/download/v4.17.1/migrate.linux-amd64.tar.gz | tar xvz
	mv migrate /usr/local/bin/

db-migrate-create:
	migrate create -ext sql -dir internal/models/migration ${table}

db-migrate-up:
	migrate -database '${uri}' -path internal/models/migration up

db-migrate-down:
	migrate -database '${uri}' -path internal/models/migration down ${N}

db-migrate-setver:
	migrate -database '${uri}' -path internal/models/migration force ${ver}