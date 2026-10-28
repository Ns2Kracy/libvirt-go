package libvirt

import (
	"runtime"
	"testing"
	"unsafe"
)

func TestDomainGetStatsOwnsDecodedValues(t *testing.T) {
	for _, invalid := range []bool{false, true} {
		name := "copied values survive native cleanup"
		if invalid {
			name = "decode failure releases native records"
		}
		t.Run(name, func(t *testing.T) {
			device := append([]byte("vda"), 0)
			params := make([]cTypedParameter, 2)
			copy(params[0].field[:], "cpu.time")
			params[0].type_ = int32(TypedParameterULongLong)
			params[0].value.bits = 123456789
			copy(params[1].field[:], "block.0.name")
			params[1].type_ = int32(TypedParameterString)
			*(*unsafe.Pointer)(unsafe.Pointer(&params[1].value)) = unsafe.Pointer(&device[0])
			if invalid {
				params[1].type_ = 999
			}
			record := cDomainStatsRecord{params: unsafe.Pointer(&params[0]), count: int32(len(params))}
			records := [2]unsafe.Pointer{unsafe.Pointer(&record), nil}
			freed := false
			api := &nativeAPI{generatedNativeAPI: generatedNativeAPI{
				virDomainListGetStats: func(_ *unsafe.Pointer, _ uint32, output *unsafe.Pointer, _ uint32) int32 {
					*output = unsafe.Pointer(&records[0])
					return 1
				},
				virDomainStatsRecordListFree: func(_ *unsafe.Pointer) {
					freed = true
					device[0] = 'X'
					params[0].value.bits = 0
				},
			}}
			handle := byte(1)
			domain := newDomain(api, unsafe.Pointer(&handle))
			stats, err := domain.GetStats(DOMAIN_STATS_CPU_TOTAL|DOMAIN_STATS_BLOCK, 0)
			runtime.KeepAlive(params)
			runtime.KeepAlive(device)
			runtime.KeepAlive(record)
			runtime.KeepAlive(records)
			if !freed {
				t.Fatal("GetStats retained native records")
			}
			if invalid {
				if err == nil {
					t.Fatal("GetStats accepted an unknown parameter type")
				}
				return
			}
			if err != nil {
				t.Fatalf("GetStats: %v", err)
			}
			if len(stats) != 2 || stats[0].Field != "cpu.time" || stats[0].Value != uint64(123456789) || stats[1].Field != "block.0.name" || stats[1].Value != "vda" {
				t.Fatalf("decoded values did not survive native cleanup: %#v", stats)
			}
		})
	}
}
