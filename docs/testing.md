# Testing

This project is experimental, Linux-only, and not production validated. See
`ci.md` for the automated Ubuntu/libvirt matrix and security workflows.

## Unit and static validation

```sh
CGO_ENABLED=0 go test ./...
CGO_ENABLED=0 go vet ./...
go test -race ./...
```

These checks do not require a running libvirt daemon. They validate the
generator, ABI layouts covered by unit tests, resource ownership helpers, and
XML fixture well-formedness. With a C compiler and libvirt headers available,
`TestNativeLayoutsMatchLibvirtHeaders` additionally compares generated sizes and
field offsets against the public C ABI. It uses `pkg-config`, or an explicit
`LIBVIRT_HEADER_DIR` pointing at the directory containing `libvirt/`.

```sh
LIBVIRT_HEADER_DIR=/path/to/include CGO_ENABLED=0 go test -run TestNativeLayoutsMatchLibvirtHeaders -v .
LIBVIRT_INTEGRATION=1 CGO_ENABLED=0 go test -run TestIntegrationTypedParameterOwnershipAndCompatibility -v .
```

The typed-parameter round trip uses the real native allocator and helper
functions. It also exercises partial-helper and pre-1.0.2 compatibility paths
without changing the process-wide loaded API.

## Synthetic libvirt integration

```sh
LIBVIRT_INTEGRATION=1 CGO_ENABLED=0 go test -run Integration -v ./integration
```

This uses `test:///default`. It does not start QEMU, connect to a production
daemon, or transfer real storage.

## Real Linux fixtures

Run only on a disposable Linux amd64 host or VM:

```sh
LIBVIRT_REAL_INTEGRATION=1 \
LIBVIRT_REAL_ALLOW_MUTATION=1 \
LIBVIRT_REAL_URI=qemu:///session \
CGO_ENABLED=0 go test -run RealIntegration -v ./integration
```

Set `LIBVIRT_REAL_START_GUEST=1` to start the no-disk test domain and wait for a
lifecycle callback. The suite creates uniquely named resources and performs
best-effort cleanup. See `../integration/testdata/real/README.md` for the exact
fixtures and safety notes.

The domain fixture (including startup, scheduler read/write and statistics)
has been executed on Linux with libvirt/QEMU 10.2.0. See
[the API audit validation record](api-audit-validation.md). The broader real
fixture matrix still requires its provisioned environment. macOS was used for
generator, native-fixture and C-header ABI checks; it is not a supported QEMU
service target.
