# Real libvirt fixtures

These XML templates are used only by `TestRealIntegrationFixtures`. They create
uniquely named, temporary resources on an explicitly selected Linux libvirt
connection and attempt cleanup with `t.Cleanup`.

The test is intentionally disabled unless all of the following are set:

```sh
LIBVIRT_REAL_INTEGRATION=1 \
LIBVIRT_REAL_ALLOW_MUTATION=1 \
LIBVIRT_REAL_URI=qemu:///session \
CGO_ENABLED=0 go test -run RealIntegration -v ./...
```

Use a disposable Linux host or VM. Do not point this test at a production
hypervisor. The test defines a domain, isolated network, directory storage pool,
small volume, ephemeral test secret, and empty network filter when their
drivers are available. It lists host interfaces and node devices read-only.

Set `LIBVIRT_REAL_START_GUEST=1` to start the no-disk fixture guest and exercise
a real QEMU lifecycle event. Without that extra flag, the domain is defined and
inspected but never started.

Cleanup is best effort. If the process is killed, resources prefixed with
`libvirt-go-` and the temporary pool directory may need manual removal.
