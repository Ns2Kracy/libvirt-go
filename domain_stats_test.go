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
				if err == nil || stats != nil {
					t.Fatal("GetStats accepted an unknown parameter type")
				}
				return
			}
			if err != nil {
				t.Fatalf("GetStats: %v", err)
			}
			if stats.Cpu == nil || !stats.Cpu.TimeSet || stats.Cpu.Time != 123456789 || len(stats.Block) != 1 || !stats.Block[0].NameSet || stats.Block[0].Name != "vda" {
				t.Fatalf("decoded values did not survive native cleanup: %#v", stats)
			}
		})
	}
}

func TestDomainStatsNumericPresenceAndSparseDevices(t *testing.T) {
	parameters := []TypedParameter{
		{Field: "state.state", Value: int32(DOMAIN_RUNNING)},
		{Field: "state.reason", Value: int32(1)},
		{Field: "cpu.time", Value: uint64(9007199254740993)},
		{Field: "cpu.user", Value: int64(12)},
		{Field: "cpu.system", Value: int64(-1)},
		{Field: "vcpu.current", Value: uint32(0)},
		{Field: "vcpu.maximum", Value: uint32(8)},
		{Field: "balloon.current", Value: uint64(1048576)},
		{Field: "block.11.rd.bytes", Value: uint64(110)},
		{Field: "block.3.wr.bytes", Value: uint64(42)},
		{Field: "block.3.wr.bytes", Value: uint64(0)},
		{Field: "block.2147483647.rd.bytes", Value: uint64(7)},
		{Field: "net.4.tx.bytes", Value: uint64(400)},
		{Field: "net.1.rx.bytes", Value: uint64(100)},
		{Field: "net.1.tx.errs", Value: int64(10)},
		{Field: "net.1.tx.errs", Value: int64(-1)},
		{Field: "block.invalid.rd.bytes", Value: uint64(999)},
		{Field: "block.-1.rd.bytes", Value: uint64(999)},
		{Field: "vcpu.0.time", Value: uint64(999)},
	}
	params := make([]cTypedParameter, len(parameters))
	for i, parameter := range parameters {
		copy(params[i].field[:], parameter.Field)
		value := unsafe.Pointer(&params[i].value)
		switch n := parameter.Value.(type) {
		case uint64:
			params[i].type_ = int32(TypedParameterULongLong)
			*(*uint64)(value) = n
		case uint32:
			params[i].type_ = int32(TypedParameterUInt)
			*(*uint32)(value) = n
		case int64:
			params[i].type_ = int32(TypedParameterLongLong)
			*(*int64)(value) = n
		case int32:
			params[i].type_ = int32(TypedParameterInt)
			*(*int32)(value) = n
		default:
			t.Fatalf("unsupported fixture value %T", n)
		}
	}
	stats, err := decodeDomainStats(unsafe.Pointer(&params[0]), int32(len(params)))
	if err != nil {
		t.Fatal(err)
	}
	if stats.State == nil || !stats.State.StateSet || stats.State.State != DOMAIN_RUNNING || !stats.State.ReasonSet || stats.State.Reason != 1 {
		t.Fatalf("state lost: %#v", stats.State)
	}
	if stats.Cpu == nil || !stats.Cpu.TimeSet || stats.Cpu.Time != 9007199254740993 || !stats.Cpu.UserSet || stats.Cpu.User != 12 || stats.Cpu.SystemSet {
		t.Fatalf("CPU width or sentinel handling changed: %#v", stats.Cpu)
	}
	if !stats.VcpuCurrentSet || stats.VcpuCurrent != 0 || !stats.VcpuMaximumSet || stats.VcpuMaximum != 8 {
		t.Fatalf("zero vCPU count became unavailable: %#v", stats)
	}
	if stats.Balloon == nil || !stats.Balloon.CurrentSet || stats.Balloon.Current != 1048576 || stats.Balloon.UnusedSet {
		t.Fatalf("missing memory counter became zero: %#v", stats.Balloon)
	}
	if len(stats.Block) != 3 || stats.Block[0].Index != 3 || !stats.Block[0].WrBytesSet || stats.Block[0].WrBytes != 0 || stats.Block[0].RdBytesSet || stats.Block[1].Index != 11 || stats.Block[1].RdBytes != 110 || stats.Block[2].Index != 2147483647 {
		t.Fatalf("sparse indices or duplicate counters changed: %#v", stats.Block)
	}
	if len(stats.Net) != 2 || stats.Net[0].Index != 1 || !stats.Net[0].RxBytesSet || stats.Net[0].RxBytes != 100 || stats.Net[0].TxErrsSet || stats.Net[1].Index != 4 || stats.Net[1].TxBytes != 400 {
		t.Fatalf("network counters changed: %#v", stats.Net)
	}
}
