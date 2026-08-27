package libvirt

import (
	"errors"
	"fmt"
	"runtime"
)

var (
	// ErrClosed indicates that a connection or domain handle has been released.
	ErrClosed = errors.New("libvirt: resource is closed")
	// ErrEmbeddedNUL indicates that a Go string cannot be represented as a C string.
	ErrEmbeddedNUL = errors.New("libvirt: string contains a NUL byte")
	// ErrLibraryNotFound indicates that the libvirt shared library could not be loaded.
	ErrLibraryNotFound = errors.New("libvirt: shared library not found")
	// ErrUnsupportedPlatform indicates that no dynamic loader is implemented for this GOOS.
	ErrUnsupportedPlatform = fmt.Errorf("libvirt: unsupported platform %s", runtime.GOOS)
)

// Error is a copy of libvirt's thread-local error record.
type Error struct {
	Operation string
	Code      int32
	Domain    int32
	Level     int32
	Message   string
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Message == "" {
		return fmt.Sprintf("libvirt: %s failed (code=%d domain=%d level=%d)", e.Operation, e.Code, e.Domain, e.Level)
	}
	return fmt.Sprintf("libvirt: %s: %s (code=%d domain=%d level=%d)", e.Operation, e.Message, e.Code, e.Domain, e.Level)
}
