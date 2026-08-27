# libvirt-go

A cgo-free Go binding for libvirt, loaded at runtime with
[`purego`](https://github.com/ebitengine/purego).

This project is an independent implementation. It does not import or wrap
`gitlab.com/libvirt/libvirt-go-module`.

## Why

- Build with `CGO_ENABLED=0`.
- No libvirt headers, `pkg-config`, or C compiler are needed at build time.
- Keep the normal in-process libvirt C API instead of reimplementing its RPC
  protocols.
- Preserve libvirt's reference-counting and structured error information in an
  idiomatic Go API.

A compatible libvirt shared library is still required at runtime. The loader
tries `libvirt.so.0`/`libvirt.so` on ELF systems and
`libvirt.0.dylib`/`libvirt.dylib` on macOS. Set `LIBVIRT_GO_LIBRARY` before the
first API call to use an explicit path.

## Generated API metadata

A shared library exposes symbol names, but it does not describe C parameter
types, enum values, or ownership rules. Generation therefore uses libvirt's
official `libvirt-api.xml`, which is produced upstream from the public headers
and installed by libvirt development packages.

`go generate ./...` resolves the XML in this order:

1. the path in `LIBVIRT_API_XML`;
2. `libvirt-api.xml` or `api/libvirt-api.xml` in this repository;
3. `/usr/share/libvirt/api/libvirt-api.xml` or the corresponding
   `/usr/local` path.

The generator parses the Go wrappers to discover every used `vir*` selector,
derives those purego signatures and the registration table from the XML, and
emits the complete raw `VIR_*` enum catalog. It also emits the idiomatic enum
aliases used by the current public API. The generated `libvirt_api.gen.go` is
committed, so package consumers still need only the runtime shared library.

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

The initial binding covers library and hypervisor versions, read-write and
read-only connections, connection liveness and URI inspection, domain listing,
lookup and definition, domain identity/state/XML inspection, and basic domain
lifecycle operations.

The public API owns native references explicitly:

- call `(*Connect).Close` for every successful open;
- call `(*Domain).Free` for every domain returned by list, lookup, or define;
- do not copy `Connect` or `Domain` values after first use.

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

The dynamic loader is implemented for Linux, macOS, FreeBSD, and NetBSD. The
most important no-cgo targets are purego's tier-1 Linux amd64/arm64 and macOS
amd64/arm64 targets. Other architectures inherit purego's support level and may
need its documented compiler flags.

Foreign signatures must exactly match libvirt's C ABI. Failed calls and
`virGetLastError` are kept on one OS thread so libvirt's thread-local error
record cannot be lost to goroutine migration. Strings and arrays owned by the
caller are copied into Go memory and released with the process C allocator.

The shared library intentionally remains loaded for the process lifetime;
unloading it would invalidate the registered foreign function values.

## Validation

```sh
CGO_ENABLED=0 go test ./...
LIBVIRT_INTEGRATION=1 CGO_ENABLED=0 go test -run Integration ./...
```

The integration test uses libvirt's `test:///default` driver. The second command
requires a local libvirt runtime.

## Next areas

The generator now owns low-level signatures, registrations, and all enum values.
Expanding the binding means adding ownership-aware wrappers; their referenced
`vir*` functions are picked up automatically on the next `go generate`. The
ABI type mapper must be extended when a new API introduces structs or callback
types. Events and callbacks, typed parameters, streams, storage, networks,
secrets, and node-device wrappers are not part of this first slice.
