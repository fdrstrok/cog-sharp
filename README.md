# cog-sharp

An experimental, community-maintained C# language plugin for
[Grafana Cog](https://github.com/grafana/cog).

The repository is named **cog-sharp**. The executable is **cog-csharp**,
because Cog discovers the `csharp` language as `cog-csharp`.

This builds on [Cog PR #1098](https://github.com/grafana/cog/pull/1098)
using the external language-plugin support introduced by
[PR #1224](https://github.com/grafana/cog/pull/1224). It uses released Cog
APIs rather than maintaining a Cog fork.

**This is not an official Grafana Foundation SDK or a production-ready
C# SDK.** The compiled example is tested; full Foundation SDK compilation,
variant runtime support, and NuGet delivery remain work in progress.

## Quick start

Requirements: Go 1.26 or newer and .NET 10 for the C# example.
Cog compatibility is currently tested with **v0.2.4**.

```sh
git clone https://github.com/fdrstrok/cog-sharp.git
cd cog-sharp
make check
make smoke
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

DTOs use public properties. With JSON generation enabled, properties use
schema JSON names and optional null properties are omitted. Explicit false
and zero values are retained. This is not a general three-state
absent/null/value model.

`output.converters` means generating builder source from objects; it is
different from JSON serialization and is not implemented. API reference
generation and extra-file templates are also not implemented.

## Foundation SDK work

The generator can consume an overlay of the upstream Foundation SDK
pipeline. Preserve the **whole pipeline**, including global transforms,
kind-registry inputs, and builder transformations, not just `units_from`.
Pin Cog, this plugin, the SDK input commit, and kind-registry inputs.

Current limitations:

- The full Foundation SDK is not yet a compiling, supported C# package.
- Composable variant interfaces and query/viz polymorphic runtime support
  are incomplete.
- Convenience transformations scoped to `[go, java]` do not automatically
  apply to `csharp`; C# needs a selective overlay.
- Full fixture parity through RPC, SDK examples, and NuGet packaging are
  still needed.
- JSON support outside the tested shapes is experimental.
- Cog v0.2.4's discriminator inference does not handle unions containing
  only foreign-package references; those require explicit schema mapping.

The first SDK milestone is a dependency-complete Dashboardv2/Common/
Timeseries/Prometheus slice that compiles and passes actual JSON and
consumer tests. Broader SDK coverage and official upstream integration
are separate milestones.

## License

Apache-2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE) for source attribution.
