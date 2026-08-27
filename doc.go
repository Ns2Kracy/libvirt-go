// Package libvirt provides a cgo-free Go binding to the libvirt C API.
//
// The package resolves libvirt at runtime and calls it through purego. It does
// not require libvirt headers, pkg-config, or a C compiler at build time. A
// compatible libvirt shared library is still required when the API is used.
// RawAPI exposes every function from the generated main libvirt API metadata;
// ownership-aware Connect and Domain wrappers provide the idiomatic subset.
package libvirt
