package libvirt

import (
	"fmt"
	"io"
	"runtime"
	"unsafe"
)

const streamWouldBlock = int32(-2)

// Stream is a reference-counted libvirt data stream.
type Stream struct {
	object nativeObject
}

func streamObject(stream *Stream) *nativeObject {
	if stream == nil {
		return nil
	}
	return &stream.object
}

func newStream(api *nativeAPI, ptr unsafe.Pointer) *Stream {
	return &Stream{object: newNativeObject(api, ptr, "stream")}
}

// NewStream creates a stream associated with this connection.
func (c *Connect) NewStream(flags uint32) (*Stream, error) {
	ptr, err := connectCall(c, "virStreamNew", func(api *nativeAPI, conn unsafe.Pointer) (unsafe.Pointer, bool) {
		result := api.virStreamNew(conn, flags)
		return result, result == nil
	})
	if err != nil {
		return nil, err
	}
	return newStream(c.api, ptr), nil
}

// Ref adds a stream reference and returns an independently freeable wrapper.
func (stream *Stream) Ref() (*Stream, error) {
	type reference struct {
		api *nativeAPI
		ptr unsafe.Pointer
	}
	ref, err := objectCall(streamObject(stream), "virStreamRef", func(api *nativeAPI, ptr unsafe.Pointer) (reference, bool) {
		result := api.virStreamRef(ptr)
		return reference{api: api, ptr: ptr}, result < 0
	})
	if err != nil {
		return nil, err
	}
	return newStream(ref.api, ref.ptr), nil
}

// Free releases this wrapper's stream reference.
func (stream *Stream) Free() error {
	return objectFree(streamObject(stream), "virStreamFree", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virStreamFree(ptr)
	})
}

// Abort aborts stream I/O.
func (stream *Stream) Abort() error {
	return objectStatus(streamObject(stream), "virStreamAbort", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virStreamAbort(ptr)
	})
}

// Finish indicates successful completion of stream I/O.
func (stream *Stream) Finish() error {
	return objectStatus(streamObject(stream), "virStreamFinish", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virStreamFinish(ptr)
	})
}

// Write sends bytes to the stream.
func (stream *Stream) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}
	var dataPtr *byte
	if len(data) != 0 {
		dataPtr = &data[0]
	}
	result, err := objectCall(streamObject(stream), "virStreamSend", func(api *nativeAPI, ptr unsafe.Pointer) (int32, bool) {
		value := api.virStreamSend(ptr, dataPtr, uintptr(len(data)))
		return value, value == -1
	})
	runtime.KeepAlive(data)
	if err != nil {
		return 0, err
	}
	if result == streamWouldBlock {
		return 0, ErrStreamWouldBlock
	}
	return int(result), nil
}

// Read receives bytes from the stream. A zero-byte native result maps to io.EOF.
func (stream *Stream) Read(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}
	result, err := objectCall(streamObject(stream), "virStreamRecv", func(api *nativeAPI, ptr unsafe.Pointer) (int32, bool) {
		value := api.virStreamRecv(ptr, &data[0], uintptr(len(data)))
		return value, value == -1
	})
	runtime.KeepAlive(data)
	if err != nil {
		return 0, err
	}
	if result == streamWouldBlock {
		return 0, ErrStreamWouldBlock
	}
	if result == 0 {
		return 0, io.EOF
	}
	return int(result), nil
}

// SendHole sends a sparse-stream hole.
func (stream *Stream) SendHole(length int64, flags uint32) error {
	return objectStatus(streamObject(stream), "virStreamSendHole", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virStreamSendHole(ptr, length, flags)
	})
}

// RecvHole receives the next sparse-stream hole length.
func (stream *Stream) RecvHole(flags uint32) (int64, error) {
	return objectCall(streamObject(stream), "virStreamRecvHole", func(api *nativeAPI, ptr unsafe.Pointer) (int64, bool) {
		var length int64
		result := api.virStreamRecvHole(ptr, &length, flags)
		return length, result < 0
	})
}

// Download starts downloading this volume into stream.
func (volume *StorageVol) Download(stream *Stream, offset, length uint64, flags uint32) error {
	return storageVolStreamCall(volume, stream, "virStorageVolDownload", func(api *nativeAPI, volumePtr, streamPtr unsafe.Pointer) int32 {
		return api.virStorageVolDownload(volumePtr, streamPtr, offset, length, flags)
	})
}

// Upload starts uploading stream data into this volume.
func (volume *StorageVol) Upload(stream *Stream, offset, length uint64, flags uint32) error {
	return storageVolStreamCall(volume, stream, "virStorageVolUpload", func(api *nativeAPI, volumePtr, streamPtr unsafe.Pointer) int32 {
		return api.virStorageVolUpload(volumePtr, streamPtr, offset, length, flags)
	})
}

func storageVolStreamCall(volume *StorageVol, stream *Stream, operation string, call func(*nativeAPI, unsafe.Pointer, unsafe.Pointer) int32) error {
	volumeObject := storageVolObject(volume)
	streamObject := streamObject(stream)
	if volumeObject == nil || streamObject == nil {
		return fmt.Errorf("%w: storage volume or stream", ErrClosed)
	}
	volumeObject.mu.RLock()
	defer volumeObject.mu.RUnlock()
	streamObject.mu.RLock()
	defer streamObject.mu.RUnlock()
	if volumeObject.ptr == nil || streamObject.ptr == nil {
		return fmt.Errorf("%w: storage volume or stream", ErrClosed)
	}
	_, err := nativeCall(volumeObject.api, operation, func() (int32, bool) {
		result := call(volumeObject.api, volumeObject.ptr, streamObject.ptr)
		return result, result < 0
	})
	return err
}
