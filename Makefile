COG_VERSION := v0.2.4
DOTNET ?= dotnet
PLUGIN := $(CURDIR)/bin/cog-csharp

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
	$(DOTNET) build sdk/Grafana.Foundation/Grafana.Foundation.csproj --configuration Release --nologo

sdk-test: sdk-build
	$(DOTNET) run --project examples/foundation/Foundation.csproj --configuration Release -- "$(CURDIR)/.cache/dashboard.json"
	go run ./cmd/sdk-validate "$(CURDIR)/.cache/dashboard.json"

sdk-pack: sdk-test
	$(DOTNET) pack sdk/Grafana.Foundation/Grafana.Foundation.csproj --configuration Release --no-build --nologo --output "$(CURDIR)/.cache/packages"
