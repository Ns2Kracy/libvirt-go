package libvirt

import (
	"bytes"
	"cmp"
	"fmt"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"unsafe"
)

// DomainStats contains owned state, total CPU, balloon, vCPU counts, block and
// interface statistics. Set flags distinguish unavailable counters from zero.
// Device slices are sorted by native index without allocating entries for gaps.
type DomainStats struct {
	State          *DomainStatsState
	Cpu            *DomainStatsCPU
	Balloon        *DomainStatsBalloon
	VcpuCurrentSet bool
	VcpuCurrent    uint64
	VcpuMaximumSet bool
	VcpuMaximum    uint64
	Block          []DomainStatsBlock
	Net            []DomainStatsNet
}

type DomainStatsState struct {
	StateSet  bool
	State     DomainState
	ReasonSet bool
	Reason    int32
}

type DomainStatsCPU struct {
	TimeSet   bool
	Time      uint64
	UserSet   bool
	User      uint64
	SystemSet bool
	System    uint64
}

type DomainStatsBalloon struct {
	CurrentSet    bool
	Current       uint64
	MaximumSet    bool
	Maximum       uint64
	AvailableSet  bool
	Available     uint64
	UnusedSet     bool
	Unused        uint64
	UsableSet     bool
	Usable        uint64
	RssSet        bool
	Rss           uint64
	LastUpdateSet bool
	LastUpdate    uint64
}

type DomainStatsBlock struct {
	Index         int
	NameSet       bool
	Name          string
	RdBytesSet    bool
	RdBytes       uint64
	WrBytesSet    bool
	WrBytes       uint64
	RdReqsSet     bool
	RdReqs        uint64
	WrReqsSet     bool
	WrReqs        uint64
	RdTimesSet    bool
	RdTimes       uint64
	WrTimesSet    bool
	WrTimes       uint64
	AllocationSet bool
	Allocation    uint64
	CapacitySet   bool
	Capacity      uint64
	PhysicalSet   bool
	Physical      uint64
}

type DomainStatsNet struct {
	Index      int
	NameSet    bool
	Name       string
	RxBytesSet bool
	RxBytes    uint64
	TxBytesSet bool
	TxBytes    uint64
	RxPktsSet  bool
	RxPkts     uint64
	TxPktsSet  bool
	TxPkts     uint64
	RxErrsSet  bool
	RxErrs     uint64
	TxErrsSet  bool
	TxErrs     uint64
	RxDropSet  bool
	RxDrop     uint64
	TxDropSet  bool
	TxDrop     uint64
}

// cDomainStatsRecord matches the public virDomainStatsRecord C ABI.
type cDomainStatsRecord struct {
	domain unsafe.Pointer
	params unsafe.Pointer
	count  int32
}

// GetStats collects DOMAIN_STATS_* groups using virDomainListGetStats and
// decodes the modeled fields directly into owned Go structures. Unmodeled
// fields, including individual vCPU counters, are ignored. Negative sentinel
// values remain unset. All native records are released before returning.
func (d *Domain) GetStats(stats, flags uint32) (*DomainStats, error) {
	var resultErr error
	result, err := domainCall(d, "virDomainListGetStats", func(api *nativeAPI, ptr unsafe.Pointer) (*DomainStats, bool) {
		if resultErr = api.requireSymbol("virDomainStatsRecordListFree"); resultErr != nil {
			return nil, false
		}
		domains := [2]unsafe.Pointer{ptr, nil}
		var records unsafe.Pointer
		count := api.virDomainListGetStats(&domains[0], stats, &records, flags)
		runtime.KeepAlive(domains)
		if count < 0 {
			return nil, true
		}
		defer api.virDomainStatsRecordListFree((*unsafe.Pointer)(records))
		if count != 1 || records == nil {
			resultErr = fmt.Errorf("libvirt: expected one domain stats record, got %d", count)
			return nil, false
		}
		record := *(**cDomainStatsRecord)(records)
		if record == nil || record.count < 0 || (record.count > 0 && record.params == nil) {
			resultErr = fmt.Errorf("libvirt: invalid domain stats record")
			return nil, false
		}
		result, decodeErr := decodeDomainStats(record.params, record.count)
		resultErr = decodeErr
		return result, false
	})
	if err != nil {
		return nil, err
	}
	return result, resultErr
}

func decodeDomainStats(memory unsafe.Pointer, count int32) (*DomainStats, error) {
	result := &DomainStats{}
	blocks, networks := map[int]int{}, map[int]int{}
	parameters := unsafe.Slice((*cTypedParameter)(memory), int(count))
	for i := range parameters {
		parameter := &parameters[i]
		end := bytes.IndexByte(parameter.field[:], 0)
		if end < 0 {
			end = len(parameter.field)
		}
		field := string(parameter.field[:end])
		var number uint64
		var available bool
		value := unsafe.Pointer(&parameter.value)
		switch TypedParameterType(parameter.type_) {
		case TypedParameterULongLong:
			number, available = parameter.value.bits, true
		case TypedParameterUInt:
			number, available = uint64(*(*uint32)(value)), true
		case TypedParameterLongLong:
			n := *(*int64)(value)
			if n >= 0 {
				number, available = uint64(n), true
			}
		case TypedParameterInt:
			n := *(*int32)(value)
			if n >= 0 {
				number, available = uint64(n), true
			}
		case TypedParameterString, TypedParameterDouble, TypedParameterBoolean:
		default:
			return nil, fmt.Errorf("libvirt: unknown typed parameter type %d for %q", parameter.type_, string(parameter.field[:end]))
		}

		group, key, _ := strings.Cut(field, ".")
		var present *bool
		var destination *uint64
		switch group {
		case "state":
			if !available {
				continue
			}
			if result.State == nil {
				result.State = &DomainStatsState{}
			}
			switch key {
			case "state":
				result.State.StateSet, result.State.State = true, DomainState(number)
			case "reason":
				result.State.ReasonSet, result.State.Reason = true, int32(number)
			}
		case "cpu":
			if result.Cpu == nil {
				result.Cpu = &DomainStatsCPU{}
			}
			switch key {
			case "time":
				present, destination = &result.Cpu.TimeSet, &result.Cpu.Time
			case "user":
				present, destination = &result.Cpu.UserSet, &result.Cpu.User
			case "system":
				present, destination = &result.Cpu.SystemSet, &result.Cpu.System
			}
		case "vcpu":
			switch key {
			case "current":
				present, destination = &result.VcpuCurrentSet, &result.VcpuCurrent
			case "maximum":
				present, destination = &result.VcpuMaximumSet, &result.VcpuMaximum
			}
		case "balloon":
			if result.Balloon == nil {
				result.Balloon = &DomainStatsBalloon{}
			}
			switch key {
			case "current":
				present, destination = &result.Balloon.CurrentSet, &result.Balloon.Current
			case "maximum":
				present, destination = &result.Balloon.MaximumSet, &result.Balloon.Maximum
			case "available":
				present, destination = &result.Balloon.AvailableSet, &result.Balloon.Available
			case "unused":
				present, destination = &result.Balloon.UnusedSet, &result.Balloon.Unused
			case "usable":
				present, destination = &result.Balloon.UsableSet, &result.Balloon.Usable
			case "rss":
				present, destination = &result.Balloon.RssSet, &result.Balloon.Rss
			case "last-update":
				present, destination = &result.Balloon.LastUpdateSet, &result.Balloon.LastUpdate
			}
		case "block", "net":
			indexText, counter, ok := strings.Cut(key, ".")
			if !ok {
				continue
			}
			index, err := strconv.Atoi(indexText)
			if err != nil || index < 0 {
				continue
			}
			if group == "block" {
				position, ok := blocks[index]
				if !ok {
					position = len(result.Block)
					blocks[index] = position
					result.Block = append(result.Block, DomainStatsBlock{Index: index})
				}
				block := &result.Block[position]
				switch counter {
				case "name":
					if TypedParameterType(parameter.type_) == TypedParameterString {
						block.Name, block.NameSet = copyCString(uintptrPointer(value)), true
					}
				case "rd.bytes":
					present, destination = &block.RdBytesSet, &block.RdBytes
				case "wr.bytes":
					present, destination = &block.WrBytesSet, &block.WrBytes
				case "rd.reqs":
					present, destination = &block.RdReqsSet, &block.RdReqs
				case "wr.reqs":
					present, destination = &block.WrReqsSet, &block.WrReqs
				case "rd.times":
					present, destination = &block.RdTimesSet, &block.RdTimes
				case "wr.times":
					present, destination = &block.WrTimesSet, &block.WrTimes
				case "allocation":
					present, destination = &block.AllocationSet, &block.Allocation
				case "capacity":
					present, destination = &block.CapacitySet, &block.Capacity
				case "physical":
					present, destination = &block.PhysicalSet, &block.Physical
				}
			} else {
				position, ok := networks[index]
				if !ok {
					position = len(result.Net)
					networks[index] = position
					result.Net = append(result.Net, DomainStatsNet{Index: index})
				}
				network := &result.Net[position]
				switch counter {
				case "name":
					if TypedParameterType(parameter.type_) == TypedParameterString {
						network.Name, network.NameSet = copyCString(uintptrPointer(value)), true
					}
				case "rx.bytes":
					present, destination = &network.RxBytesSet, &network.RxBytes
				case "tx.bytes":
					present, destination = &network.TxBytesSet, &network.TxBytes
				case "rx.pkts":
					present, destination = &network.RxPktsSet, &network.RxPkts
				case "tx.pkts":
					present, destination = &network.TxPktsSet, &network.TxPkts
				case "rx.errs":
					present, destination = &network.RxErrsSet, &network.RxErrs
				case "tx.errs":
					present, destination = &network.TxErrsSet, &network.TxErrs
				case "rx.drop":
					present, destination = &network.RxDropSet, &network.RxDrop
				case "tx.drop":
					present, destination = &network.TxDropSet, &network.TxDrop
				}
			}
		}
		if present != nil {
			*present, *destination = available, number
		}
	}
	slices.SortFunc(result.Block, func(a, b DomainStatsBlock) int { return cmp.Compare(a.Index, b.Index) })
	slices.SortFunc(result.Net, func(a, b DomainStatsNet) int { return cmp.Compare(a.Index, b.Index) })
	return result, nil
}
