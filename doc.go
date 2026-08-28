// Package libvirt provides a cgo-free Go binding to the libvirt C API.
//
// The package resolves libvirt at runtime and calls it through purego. It does
// not require libvirt headers, pkg-config, or a C compiler at build time. A
// compatible libvirt shared library is still required when the API is used.
// RawAPI exposes functions generated from the main, admin, QEMU, and LXC API
// metadata; ownership-aware wrappers provide the idiomatic resource layer.
// The project is experimental, Linux-only, and not production validated.
package libvirt
