COG_VERSION := v0.2.4
DOTNET ?= dotnet
PLUGIN := $(CURDIR)/bin/cog-csharp
SDK_VERSION ?= 0.1.0-alpha.1
SDK_SOURCE_COMMIT := $(shell git rev-parse HEAD)
SDK_BUILD_PROPERTIES := -p:Version="$(SDK_VERSION)" -p:RepositoryCommit="$(SDK_SOURCE_COMMIT)"

.PHONY: build check generate smoke sdk-generate sdk-build sdk-test sdk-pack

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
	$(DOTNET) run --project examples/basic/Basic.csproj --configuration Release

sdk-generate: build
	go run ./cmd/sdk-generate $(SDK_GENERATE_ARGS)

sdk-build: sdk-generate
	$(DOTNET) build sdk/Grafana.Foundation/Grafana.Foundation.csproj --configuration Release --nologo $(SDK_BUILD_PROPERTIES)

sdk-test: sdk-build
	$(DOTNET) run --project examples/foundation/Foundation.csproj --configuration Release $(SDK_BUILD_PROPERTIES) -- "$(CURDIR)/.cache/dashboard.json"
	go run ./cmd/sdk-validate "$(CURDIR)/.cache/dashboard.json"

sdk-pack: sdk-test
	$(DOTNET) pack sdk/Grafana.Foundation/Grafana.Foundation.csproj --configuration Release --no-build --nologo $(SDK_BUILD_PROPERTIES) --output "$(CURDIR)/.cache/packages"
	go run ./cmd/sdk-release verify --package ".cache/packages/CogSharp.Grafana.Foundation.$(SDK_VERSION).nupkg" --version "$(SDK_VERSION)" --commit "$(SDK_SOURCE_COMMIT)"
