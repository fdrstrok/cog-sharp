COG_VERSION := v0.2.4
DOTNET ?= dotnet
PLUGIN := $(CURDIR)/bin/cog-csharp

.PHONY: build check generate smoke

build:
	go build -o $(PLUGIN) ./cmd/cog-csharp

check:
	@test -z "$$(gofmt -l cmd internal)" || (gofmt -l cmd internal; exit 1)
	go test ./...
	go vet ./...

generate: build
	go run github.com/grafana/cog/cmd/cli@$(COG_VERSION) generate \
		--config examples/basic/pipeline.yaml \
		--plugin-directory "$(CURDIR)/bin"

smoke: generate
	$(DOTNET) run --project examples/basic/Basic.csproj --configuration Release --nologo
