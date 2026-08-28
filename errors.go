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
	// ErrSymbolUnavailable indicates that the loaded libvirt predates an API function.
	ErrSymbolUnavailable = errors.New("libvirt: symbol unavailable")
	// ErrStreamWouldBlock indicates nonblocking stream I/O cannot currently progress.
	ErrStreamWouldBlock = errors.New("libvirt: stream would block")
	// ErrUnsupportedPlatform indicates that no dynamic loader is implemented for this GOOS.
	ErrUnsupportedPlatform = fmt.Errorf("libvirt: unsupported platform %s", runtime.GOOS)
)

// SymbolUnavailableError describes a function absent from the loaded libvirt.
type SymbolUnavailableError struct {
	Symbol string
	Since  string
}

func (e *SymbolUnavailableError) Error() string {
	if e.Since == "" {
		return fmt.Sprintf("libvirt: symbol %s is unavailable", e.Symbol)
	}
	return fmt.Sprintf("libvirt: symbol %s requires libvirt %s or newer", e.Symbol, e.Since)
}

func (e *SymbolUnavailableError) Unwrap() error {
	return ErrSymbolUnavailable
}

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
