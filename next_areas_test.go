package libvirt

import (
	"errors"
	"io"
	"runtime"
	"testing"
	"unsafe"
)

func TestDecodeTypedParameters(t *testing.T) {
	text := append([]byte("value"), 0)
	params := make([]cTypedParameter, 3)
	copy(params[0].field[:], "limit")
	params[0].type_ = int32(TypedParameterLongLong)
	*(*int64)(unsafe.Pointer(&params[0].value)) = -42
	copy(params[1].field[:], "enabled")
	params[1].type_ = int32(TypedParameterBoolean)
	*(*byte)(unsafe.Pointer(&params[1].value)) = 1
	copy(params[2].field[:], "mode")
	params[2].type_ = int32(TypedParameterString)
	*(*unsafe.Pointer)(unsafe.Pointer(&params[2].value)) = unsafe.Pointer(&text[0])

	decoded, err := decodeTypedParameters(unsafe.Pointer(&params[0]), int32(len(params)))
	runtime.KeepAlive(text)
	if err != nil {
		t.Fatal(err)
	}
	if decoded[0].Field != "limit" || decoded[0].Value != int64(-42) {
		t.Fatalf("decoded integer = %#v", decoded[0])
	}
	if decoded[1].Field != "enabled" || decoded[1].Value != true {
		t.Fatalf("decoded boolean = %#v", decoded[1])
	}
	if decoded[2].Field != "mode" || decoded[2].Value != "value" {
		t.Fatalf("decoded string = %#v", decoded[2])
	}
}

func TestTypedParameterLayoutLinuxAMD64(t *testing.T) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("layout assertion is for the supported Linux amd64 target")
	}
	var param cTypedParameter
	if size := unsafe.Sizeof(param); size != 96 {
		t.Fatalf("sizeof(cTypedParameter) = %d, want 96", size)
	}
	if offset := unsafe.Offsetof(param.value); offset != 88 {
		t.Fatalf("offsetof(value) = %d, want 88", offset)
	}
}

func TestStreamWouldBlockAndEOF(t *testing.T) {
	handleValue := byte(1)
	api := &nativeAPI{
		generatedNativeAPI: generatedNativeAPI{
			virResetLastError: func() {},
			virStreamSend: func(unsafe.Pointer, *byte, uintptr) int32 {
				return streamWouldBlock
			},
			virStreamRecv: func(unsafe.Pointer, *byte, uintptr) int32 {
				return 0
			},
		},
	}
	stream := newStream(api, unsafe.Pointer(&handleValue))
	if _, err := stream.Write([]byte("x")); !errors.Is(err, ErrStreamWouldBlock) {
		t.Fatalf("Stream.Write error = %v, want ErrStreamWouldBlock", err)
	}
	if _, err := stream.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
		t.Fatalf("Stream.Read error = %v, want io.EOF", err)
	}
}

func TestNetworkFreeConsumesReferenceOnce(t *testing.T) {
	handleValue := byte(1)
	calls := 0
	api := &nativeAPI{
		generatedNativeAPI: generatedNativeAPI{
			virResetLastError: func() {},
			virNetworkFree: func(unsafe.Pointer) int32 {
				calls++
				return 0
			},
		},
	}
	network := newNetwork(api, unsafe.Pointer(&handleValue))
	if err := network.Free(); err != nil {
		t.Fatal(err)
	}
	if err := network.Free(); !errors.Is(err, ErrClosed) {
		t.Fatalf("second Network.Free error = %v, want ErrClosed", err)
	}
	if calls != 1 {
		t.Fatalf("virNetworkFree called %d times, want 1", calls)
	}
}

func TestCallbackPanicIsContained(t *testing.T) {
	called := false
	invokeCallback(func() {
		called = true
		panic("callback failure")
	})
	if !called {
		t.Fatal("callback was not invoked")
	}
}
