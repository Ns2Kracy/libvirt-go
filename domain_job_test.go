package libvirt

import (
	"runtime"
	"testing"
	"unsafe"
)

func TestDomainGetJobStatsDecodesCompletion(t *testing.T) {
	message := append([]byte("disk full"), 0)
	rawParams := make([]cTypedParameter, 3)
	copy(rawParams[0].field[:], domainJobOperationField)
	rawParams[0].type_ = int32(TypedParameterInt)
	*(*int32)(unsafe.Pointer(&rawParams[0].value)) = int32(DomainJobOperationBackup)
	copy(rawParams[1].field[:], domainJobSuccessField)
	rawParams[1].type_ = int32(TypedParameterBoolean)
	*(*byte)(unsafe.Pointer(&rawParams[1].value)) = 0
	copy(rawParams[2].field[:], domainJobErrorField)
	rawParams[2].type_ = int32(TypedParameterString)
	*(*unsafe.Pointer)(unsafe.Pointer(&rawParams[2].value)) = unsafe.Pointer(&message[0])

	freed := false
	api := &nativeAPI{generatedNativeAPI: generatedNativeAPI{
		virDomainGetJobStats: func(_ unsafe.Pointer, jobType *int32, params *unsafe.Pointer, count *int32, _ uint32) int32 {
			*jobType = int32(DomainJobCompleted)
			*params = unsafe.Pointer(&rawParams[0])
			*count = int32(len(rawParams))
			return 0
		},
		virTypedParamsFree: func(params unsafe.Pointer, count int32) {
			if params != unsafe.Pointer(&rawParams[0]) || count != int32(len(rawParams)) {
				t.Fatalf("virTypedParamsFree(%p, %d) received unexpected storage", params, count)
			}
			freed = true
		},
	}}
	handle := byte(1)
	domain := &Domain{api: api, ptr: unsafe.Pointer(&handle)}

	stats, err := domain.GetJobStats(0)
	runtime.KeepAlive(rawParams)
	runtime.KeepAlive(message)
	if err != nil {
		t.Fatalf("GetJobStats: %v", err)
	}
	if stats.Type != DomainJobCompleted || stats.Operation == nil || *stats.Operation != DomainJobOperationBackup {
		t.Fatalf("GetJobStats operation = %#v", stats)
	}
	if stats.Success == nil || *stats.Success || stats.ErrorMessage != "disk full" {
		t.Fatalf("GetJobStats completion = %#v", stats)
	}
	if !freed {
		t.Fatal("GetJobStats did not free native typed parameters")
	}
}
