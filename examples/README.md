# Examples

Each subdirectory is a buildable command that demonstrates the ownership-aware
high-level API. The commands default to libvirt's read-only synthetic test URI
when that is appropriate, and all accept `-h` for their full usage.

A compatible libvirt shared library is required at runtime even though the
examples build with `CGO_ENABLED=0`.

## Quick start

```sh
CGO_ENABLED=0 go run ./examples/inventory -uri test:///default
```

## Catalog

| Example | Access | Operations |
| --- | --- | --- |
| [`inventory`](inventory) | Read-only | Open and inspect a connection; list domains, networks, and storage pools. |
| [`domain-lifecycle`](domain-lifecycle) | Mixed | List, inspect, define, start, shut down, destroy, and undefine domains. |
| [`domain-snapshots`](domain-snapshots) | Mixed | List, create, inspect, revert, and delete snapshots; manage checkpoints and child checkpoints. |
| [`typed-parameters`](typed-parameters) | Mixed | Get or set domain memory, NUMA, scheduler, and block-I/O typed parameters. |

## Resource ownership

Every successful connection open is paired with `Close`, and every returned
resource handle is paired with `Free`. Examples that register callbacks also
close the callback handle before closing the connection.

## Mutation safety

Examples marked as mutating require an explicit action and the identifiers or
XML documents needed by that action. Run them only against a disposable host or
VM until the project is production-ready. Prefer `qemu:///session` over
`qemu:///system` while experimenting, and inspect every XML document before
passing it to libvirt.
