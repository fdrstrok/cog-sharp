# cog-sharp

An experimental, community-maintained C# language plugin for
[Grafana Cog](https://github.com/grafana/cog).

The repository is named **cog-sharp**. The executable is **cog-csharp**,
because Cog discovers the `csharp` language as `cog-csharp`.

This builds on [Cog PR #1098](https://github.com/grafana/cog/pull/1098)
using the external language-plugin support introduced by
[PR #1224](https://github.com/grafana/cog/pull/1224). It uses released Cog
APIs rather than maintaining a Cog fork.

**This is an independent community project, not an official Grafana SDK.**
The complete pinned Foundation SDK schema pipeline generates a .NET 10
library that builds with warnings treated as errors. Real Dashboardv2 and
legacy dashboard consumers, typed query/viz JSON, and local NuGet packaging
are tested. Broader runtime coverage and package publication remain experimental.

## Quick start

Requirements: Go 1.26 or newer and .NET 10 for the C# example.
Git is also required for the pinned SDK inputs.
Cog compatibility is currently tested with **v0.2.4**.

```sh
git clone https://github.com/fdrstrok/cog-sharp.git
cd cog-sharp
make check
make smoke
make sdk-test
```

`make smoke` builds the plugin, invokes released Cog through its real RPC
plugin boundary, generates C#, and compiles and runs the example with
nullable warnings treated as errors. It checks fluent builders, default
`System.Text.Json` serialization, optional false/zero values, keyword
identifiers, and scalar and discriminated unions.
The example also compiles foreign-package builder references without
implicit usings and checks regexp patterns containing backslashes and quotes.

To install the plugin from source:

```sh
go install github.com/fdrstrok/cog-sharp/cmd/cog-csharp@latest
```

Ensure your Go binary directory is on `PATH`, or supply its absolute path
with Cog's `--plugin-directory` flag.

## Pipeline configuration

Keep your existing Cog inputs and transformations. Add the plugin under
`output.language_plugins`, not `output.languages`:

```yaml
output:
  directory: generated/%l
  types: true
  builders: true
  converters: false
  api_reference: false
  language_plugins:
    csharp:
      namespace_root: Grafana.Foundation
      generate_json_converters: true
```

```sh
cog generate --config pipeline.yaml --plugin-directory /absolute/path/to/bin
```

Generated files live under `src/<namespace>/<package>/`. Supply a .NET 10
`.csproj` as shown in [`examples/basic`](examples/basic).
The generated SDK consumer does not need Go or Cog at runtime.

| Option | Default | Meaning |
|---|---|---|
| `namespace_root` | `Grafana.Foundation` | Root namespace for generated C# |
| `generate_json_converters` | `false` | JSON names, string enums, and union converters |
| `skip_runtime` | `false` | Raw types only; incompatible with builders and JSON converters |
| `generate_equality` | `false` | Not implemented; requesting it returns an error |
| `overrides_templates` | unset | One directory containing template overrides; use a scalar path for Cog RPC |

DTOs use public properties. With JSON generation enabled, properties use
schema JSON names and optional null properties are omitted. Explicit false
and zero values are retained. This is not a general three-state
absent/null/value model.

`output.converters` means generating builder source from objects; it is
different from JSON serialization and is not implemented. API reference
generation and extra-file templates are also not implemented.

## Build the C# Foundation SDK

```sh
make sdk-build  # fetch pinned inputs, generate all packages, build the library
make sdk-test   # also run real consumers and validate dashboard JSON
make sdk-pack   # also create a local prerelease .nupkg
```

The project is [`sdk/Grafana.Foundation/Grafana.Foundation.csproj`](sdk/Grafana.Foundation/Grafana.Foundation.csproj).
Generated source is ignored by git. It must be generated before a direct
`dotnet build`; the project rejects missing output rather than building an
empty library.

The DLL is written to `sdk/Grafana.Foundation/bin/Release/net10.0/`.
Local packages are written to `.cache/packages/` with the community package
ID `CogSharp.Grafana.Foundation`. No package is published by these commands.
Only regeneration needs Go/Cog and network access; C# consumers use normal
project or package references.

Generation copies the **whole upstream pipeline**, including global
transforms, kind-registry inputs, and builder transformations, into an
ignored overlay. Java-compatible no-union conveniences are enabled for C#;
SDK-specific layout and query-refId hooks live in `sdk/templates/overrides`.
Original upstream checkouts are not modified.

| Input | Pin |
|---|---|
| Cog | `v0.2.4` |
| Foundation SDK | `72b873117be02d3cd352c4ac8590b6555b0d1ce2` |
| Kind registry | `f5c7186c5a8c568155e156a3d41c8bdb80bc53ae` |

These pins live in `internal/sdkgen/generate.go`. Referenced Grafana schema
versions come from the pinned SDK configuration. Input provenance is written
beside the generated sources.

[`examples/foundation`](examples/foundation) builds Dashboardv2 with
Timeseries and Prometheus builders. Its executable regression checks cover:

- Typed query, visualization-options, and field-config deserialization.
- Unknown plugin JSON preservation and known-plugin wire round trips.
- Automatic query refIds and legacy dashboard panel/row positioning.
- Numeric and empty-string enums, schema defaults, and nested collections of builders.

The example's JSON is then validated against the pinned upstream
Dashboardv2 JSON Schema. This validates the dashboard specification, not a
Grafana HTTP resource envelope or a live server deployment.

For existing input checkouts, pass
`SDK_GENERATE_ARGS="--foundation-sdk-dir /path/to/sdk --kind-registry-dir /path/to/registry"`.
The revisions and relevant input worktrees must match the pins.

Current limitations:

- Compiling every generated package does not prove every builder or plugin's runtime behavior.
- Full upstream fixture parity through RPC and wider plugin-specific JSON tests are still needed.
- The default variant registry is generated from the pinned schemas; unknown plugin specs remain raw JSON.
- Equality, object-to-builder-source converters, API reference generation, and extra-file templates remain unsupported.
- NuGet publication, official Grafana maintenance, live Grafana deployment, and Native AOT are not claimed.
- Cog v0.2.4's discriminator inference does not handle unions containing
  only foreign-package references; those require explicit schema mapping.

## License

Apache-2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE) for source attribution.
