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
XML fixture well-formedness.

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

The real suite has not been executed in this development environment because no
QEMU binary or libvirt daemon is installed. macOS, FreeBSD, and NetBSD are not
tested or supported.
