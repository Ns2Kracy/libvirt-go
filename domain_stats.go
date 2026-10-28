package libvirt

import (
	"fmt"
	"runtime"
	"unsafe"
)

// cDomainStatsRecord matches the public virDomainStatsRecord C ABI.
type cDomainStatsRecord struct {
	domain unsafe.Pointer
	params unsafe.Pointer
	count  int32
}

// GetStats collects the requested DOMAIN_STATS_* groups for this domain using
// virDomainListGetStats. Values keep libvirt's field names and units. Unsupported
// fields are absent unless CONNECT_GET_ALL_DOMAINS_STATS_ENFORCE_STATS is set.
// All returned parameters are copied into Go memory and native records are freed.
func (d *Domain) GetStats(stats, flags uint32) ([]TypedParameter, error) {
	var resultErr error
	parameters, err := domainCall(d, "virDomainListGetStats", func(api *nativeAPI, ptr unsafe.Pointer) ([]TypedParameter, bool) {
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
		parameters, decodeErr := decodeTypedParameters(record.params, record.count)
		resultErr = decodeErr
		return parameters, false
	})
	if err != nil {
		return nil, err
	}
	return parameters, resultErr
}
