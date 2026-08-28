# Architecture

The repository follows Go package boundaries rather than placing every feature
in its own directory.

## Layout

```text
.
├── .github/workflows/         CI and CodeQL automation
├── api/                       vendored libvirt API XML metadata
├── ci/                        isolated libvirt CI daemon config
├── cmd/libvirt-api-gen/       generator executable
├── docs/                      architecture and testing documentation
├── examples/list-domains/     buildable public-API example
├── integration/               external black-box integration test package
│   └── testdata/real/         gated, mutating real-libvirt XML fixtures
├── internal/generator/        XML parsing and template-driven generation
│   └── templates/             generated Go source templates
├── libvirt_api.gen.go         generated public RawAPI and symbol catalog
└── *.go                       public package libvirt
```

## Why the public files remain at the root

Go defines a package by directory. Methods for `Connect`, `Domain`, `Network`,
and the other public handle types must be declared in the same package as those
types. Moving each resource into a separate directory would create different Go
packages, change public import paths, and force a compatibility facade or create
import cycles.

The root is therefore intentionally the single high-level `libvirt` package.
Directory boundaries are used where they are meaningful:

- CI automation and its isolated daemon configuration belong in `.github/` and `ci/`;
- executable tooling belongs in `cmd/`;
- non-public generator implementation and embedded source templates belong in `internal/`;
- black-box and mutating tests belong in `integration/`;
- user examples belong in `examples/`;
- long-form project documentation belongs in `docs/`.

The generated file remains in the root because it contributes methods to the
public `RawAPI` type and generated declarations used by the high-level package.
