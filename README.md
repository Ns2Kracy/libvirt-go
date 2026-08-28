# libvirt-go

A cgo-free Go binding for libvirt, loaded at runtime with
[`purego`](https://github.com/ebitengine/purego).

This project is an independent implementation. It does not import or wrap
`gitlab.com/libvirt/libvirt-go-module`.

> [!WARNING]
> This project is experimental. It has only been exercised on Linux with
> libvirt's synthetic `test:///default` driver. It has not been validated in a
> production libvirt/QEMU/KVM environment and is not currently recommended for
> production workloads. macOS, FreeBSD, and NetBSD are untested and unsupported.

## Why

- Build with `CGO_ENABLED=0`.
- No libvirt headers, `pkg-config`, or C compiler are needed at build time.
- Keep the normal in-process libvirt C API instead of reimplementing its RPC
  protocols.
- Preserve libvirt's reference-counting and structured error information in an
  idiomatic Go API.

A compatible Linux libvirt shared library is still required at runtime. The
loader tries `libvirt.so.0`/`libvirt.so`; set `LIBVIRT_GO_LIBRARY` before the
first API call to use an explicit path. Admin, QEMU, and LXC extension libraries
can be overridden with `LIBVIRT_ADMIN_LIBRARY`, `LIBVIRT_QEMU_LIBRARY`, and
`LIBVIRT_LXC_LIBRARY`.

## Repository layout

- `cmd/libvirt-api-gen/`: generator executable;
- `internal/generator/`: XML parsing, templates, and Go source generation;
- `integration/`: black-box synthetic and gated real-libvirt tests;
- `integration/testdata/real/`: mutating real Linux XML fixtures;
- `examples/`: buildable public API examples;
- `docs/`: architecture and testing details.

Public package files remain at the repository root because Go package and method
boundaries follow directories. See `docs/architecture.md` for the rationale.

## Generated API metadata

A shared library exposes symbol names, but it does not describe C parameter
types, enum values, or ownership rules. Generation therefore uses libvirt's official main, admin, QEMU, and
LXC API XML files, which are produced upstream from the public headers and
installed by libvirt development packages.

`go generate ./...` resolves the XML in this order:

1. the path in `LIBVIRT_API_XML`;
2. `libvirt-api.xml` or `api/libvirt-api.xml` in this repository;
3. `/usr/share/libvirt/api/libvirt-api.xml` or the corresponding
   `/usr/local` path.

The admin, LXC, and QEMU XML files must be present beside the resolved main XML
file. Their runtime shared libraries are optional; absent extension libraries
make only their corresponding symbols unavailable.

The generator emits every function declared by all four API XML files (567
functions and 1,071 enums for the installed libvirt 11.6 metadata), including
purego signatures, introduction versions, source-library routing, the symbol
registration table, and public `RawAPI.Vir*` methods. It also emits idiomatic
enum aliases used by the high-level API.
The generated `libvirt_api.gen.go` is committed, so package consumers still
need only the runtime shared library.

Generated symbols are optional at load time. This lets output generated from a
new libvirt XML load an older libvirt shared library: symbol presence is checked
with `dlsym`, missing functions are recorded, and a high-level call returns a
`SymbolUnavailableError` wrapping `ErrSymbolUnavailable` instead of calling
a nil function pointer. `HasSymbol` reports runtime availability and
`SymbolVersion` reports the upstream introduction version. Symbol presence,
rather than only a numeric version comparison, also supports distribution
backports. Both high-level and generated raw calls return the compatibility
error before touching a missing function pointer.

Raw methods otherwise preserve C return values and ownership rules. Their
`error` result only represents symbol availability, so callers must still
check libvirt's C failure sentinel and manage native resources. Lock the
goroutine with `runtime.LockOSThread` when a raw failing call must be paired
with `RawAPI.VirGetLastError`.

To update against a libvirt source checkout or a specific installed version:

```sh
LIBVIRT_API_XML=/path/to/libvirt-api.xml go generate ./...
CGO_ENABLED=0 go test ./...
```

High-level wrappers remain handwritten because reference ownership, allocated
return values, thread-local errors, and idiomatic Go resource lifetimes are
behavioral contracts rather than facts recoverable from an ELF/Mach-O symbol
table.

## Current scope

The generated low-level surface covers the main, admin, QEMU, and LXC API XML
files. The ownership-aware high-level API covers connections, domains, networks
and ports, storage pools and volumes, secrets, node devices, host interfaces,
network filters, snapshots, checkpoints, streams, typed domain parameters, the
default event loop, connection-close callbacks, and domain lifecycle callbacks.
Other specialized operations remain available through `RawAPI`.

The public API owns native resources explicitly:

- call `(*Connect).Close` for every successful open;
- call `Free` for every returned object or stream reference;
- call callback handle `Close` methods to unregister callbacks;
- do not copy handle values after first use.

## Example

```go
package main

import (
 "fmt"
 "log"

 libvirt "github.com/Ns2Kracy/libvirt-go"
)

func main() {
 raw, err := libvirt.GetVersion()
 if err != nil {
  log.Fatal(err)
 }
 fmt.Println("libvirt", libvirt.DecodeVersion(raw))

 conn, err := libvirt.NewConnectReadOnly("test:///default")
 if err != nil {
  log.Fatal(err)
 }
 defer conn.Close()

 domains, err := conn.ListAllDomains(0)
 if err != nil {
  log.Fatal(err)
 }
 for _, domain := range domains {
  name, nameErr := domain.GetName()
  if freeErr := domain.Free(); freeErr != nil {
   log.Printf("free domain: %v", freeErr)
  }
  if nameErr != nil {
   log.Fatal(nameErr)
  }
  fmt.Println(name)
 }
}
```

Run it without cgo:

```sh
CGO_ENABLED=0 go run ./path/to/your/program
```

## Platform and ABI notes

Only Linux amd64 is currently runtime-tested and supported. Linux arm64 has only
been compile-checked. The macOS, FreeBSD, and NetBSD loader code has not been
tested by this project and must be treated as unsupported.

Foreign signatures must exactly match libvirt's C ABI. Failed calls and
`virGetLastError` are kept on one OS thread so libvirt's thread-local error
record cannot be lost to goroutine migration. Strings and arrays owned by the
caller are copied into Go memory and released with the process C allocator.

The shared library intentionally remains loaded for the process lifetime;
unloading it would invalidate the registered foreign function values.

## Validation

```sh
CGO_ENABLED=0 go test ./...
LIBVIRT_INTEGRATION=1 CGO_ENABLED=0 go test -run Integration ./integration
```

The integration test uses libvirt's synthetic `test:///default` driver on Linux.
It exercises resource ownership, typed parameters, callback registration, and
main/admin/QEMU/LXC symbol loading.

Real Linux fixtures live in `integration/testdata/real` and are protected by explicit
mutation gates:

```sh
LIBVIRT_REAL_INTEGRATION=1 \
LIBVIRT_REAL_ALLOW_MUTATION=1 \
LIBVIRT_REAL_URI=qemu:///session \
CGO_ENABLED=0 go test -run RealIntegration -v ./integration
```

Set `LIBVIRT_REAL_START_GUEST=1` to additionally start the no-disk fixture VM
and wait for a lifecycle callback. Run this only on a disposable Linux host or
VM; see `integration/testdata/real/README.md`. This development machine has no QEMU binary
or libvirt daemon, so the real fixture suite has been compiled but not executed.
No production-environment validation has been performed.

## Next areas

Before a stable release, the real fixture suite must be run in disposable
libvirtd/virtqemud environments with QEMU/KVM, storage/stream transfers and
callback delivery need stress coverage, and the binding needs race/fuzz tests,
security review, and validation against multiple old and new libvirt versions.
