# Libvirt API metadata

This directory vendors the generated API XML used by `go generate`.

- Upstream release: libvirt 12.6.0
- Git tag: `v12.6.0`
- Tag commit: `4bc0719ff048d821657016b07e7ab74e94333b01`
- Release archive: `https://download.libvirt.org/libvirt-12.6.0.tar.xz`
- Release archive SHA-256:
  `1592256deb76fc94028ff083a4d9f06a74f3b92a66a1794f37bc26f21430c888`

The upstream repository does not commit these generated files. They were
produced with `scripts/apibuild.py` from the tagged source. Before running the
builder, `include/libvirt/libvirt-common.h.in` was configured as
`build/include/libvirt/libvirt-common.h` with
`LIBVIRT_VERSION_NUMBER=12006000`; this preserves the public typed-parameter
functions declared by the configured common header.

The four files correspond to the main, admin, LXC, and QEMU shared libraries.
The binding generator hashes all four documents and records the combined digest
in `libvirt_api.gen.go`.
