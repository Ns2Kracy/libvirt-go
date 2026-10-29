package libvirt

import (
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
		parameter, err := decodeTypedParameter(&parameters[i])
		if err != nil {
			return nil, err
		}
		field := parameter.Field
		var number uint64
		var available bool
		switch value := parameter.Value.(type) {
		case uint64:
			number, available = value, true
		case uint32:
			number, available = uint64(value), true
		case int64:
			if value >= 0 {
				number, available = uint64(value), true
			}
		case int32:
			if value >= 0 {
				number, available = uint64(value), true
			}
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
			switch field {
			case DOMAIN_STATS_STATE_STATE:
				result.State.StateSet, result.State.State = true, DomainState(number)
			case DOMAIN_STATS_STATE_REASON:
				result.State.ReasonSet, result.State.Reason = true, int32(number)
			}
		case "cpu":
			if result.Cpu == nil {
				result.Cpu = &DomainStatsCPU{}
			}
			switch field {
			case DOMAIN_STATS_CPU_TIME:
				present, destination = &result.Cpu.TimeSet, &result.Cpu.Time
			case DOMAIN_STATS_CPU_USER:
				present, destination = &result.Cpu.UserSet, &result.Cpu.User
			case DOMAIN_STATS_CPU_SYSTEM:
				present, destination = &result.Cpu.SystemSet, &result.Cpu.System
			}
		case "vcpu":
			switch field {
			case DOMAIN_STATS_VCPU_CURRENT:
				present, destination = &result.VcpuCurrentSet, &result.VcpuCurrent
			case DOMAIN_STATS_VCPU_MAXIMUM:
				present, destination = &result.VcpuMaximumSet, &result.VcpuMaximum
			}
		case "balloon":
			if result.Balloon == nil {
				result.Balloon = &DomainStatsBalloon{}
			}
			switch field {
			case DOMAIN_STATS_BALLOON_CURRENT:
				present, destination = &result.Balloon.CurrentSet, &result.Balloon.Current
			case DOMAIN_STATS_BALLOON_MAXIMUM:
				present, destination = &result.Balloon.MaximumSet, &result.Balloon.Maximum
			case DOMAIN_STATS_BALLOON_AVAILABLE:
				present, destination = &result.Balloon.AvailableSet, &result.Balloon.Available
			case DOMAIN_STATS_BALLOON_UNUSED:
				present, destination = &result.Balloon.UnusedSet, &result.Balloon.Unused
			case DOMAIN_STATS_BALLOON_USABLE:
				present, destination = &result.Balloon.UsableSet, &result.Balloon.Usable
			case DOMAIN_STATS_BALLOON_RSS:
				present, destination = &result.Balloon.RssSet, &result.Balloon.Rss
			case DOMAIN_STATS_BALLOON_LAST_UPDATE:
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
				switch "." + counter {
				case DOMAIN_STATS_BLOCK_SUFFIX_NAME:
					if name, ok := parameter.Value.(string); ok {
						block.Name, block.NameSet = name, true
					}
				case DOMAIN_STATS_BLOCK_SUFFIX_RD_BYTES:
					present, destination = &block.RdBytesSet, &block.RdBytes
				case DOMAIN_STATS_BLOCK_SUFFIX_WR_BYTES:
					present, destination = &block.WrBytesSet, &block.WrBytes
				case DOMAIN_STATS_BLOCK_SUFFIX_RD_REQS:
					present, destination = &block.RdReqsSet, &block.RdReqs
				case DOMAIN_STATS_BLOCK_SUFFIX_WR_REQS:
					present, destination = &block.WrReqsSet, &block.WrReqs
				case DOMAIN_STATS_BLOCK_SUFFIX_RD_TIMES:
					present, destination = &block.RdTimesSet, &block.RdTimes
				case DOMAIN_STATS_BLOCK_SUFFIX_WR_TIMES:
					present, destination = &block.WrTimesSet, &block.WrTimes
				case DOMAIN_STATS_BLOCK_SUFFIX_ALLOCATION:
					present, destination = &block.AllocationSet, &block.Allocation
				case DOMAIN_STATS_BLOCK_SUFFIX_CAPACITY:
					present, destination = &block.CapacitySet, &block.Capacity
				case DOMAIN_STATS_BLOCK_SUFFIX_PHYSICAL:
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
				switch "." + counter {
				case DOMAIN_STATS_NET_SUFFIX_NAME:
					if name, ok := parameter.Value.(string); ok {
						network.Name, network.NameSet = name, true
					}
				case DOMAIN_STATS_NET_SUFFIX_RX_BYTES:
					present, destination = &network.RxBytesSet, &network.RxBytes
				case DOMAIN_STATS_NET_SUFFIX_TX_BYTES:
					present, destination = &network.TxBytesSet, &network.TxBytes
				case DOMAIN_STATS_NET_SUFFIX_RX_PKTS:
					present, destination = &network.RxPktsSet, &network.RxPkts
				case DOMAIN_STATS_NET_SUFFIX_TX_PKTS:
					present, destination = &network.TxPktsSet, &network.TxPkts
				case DOMAIN_STATS_NET_SUFFIX_RX_ERRS:
					present, destination = &network.RxErrsSet, &network.RxErrs
				case DOMAIN_STATS_NET_SUFFIX_TX_ERRS:
					present, destination = &network.TxErrsSet, &network.TxErrs
				case DOMAIN_STATS_NET_SUFFIX_RX_DROP:
					present, destination = &network.RxDropSet, &network.RxDrop
				case DOMAIN_STATS_NET_SUFFIX_TX_DROP:
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
