package libvirt

import (
	"fmt"
	"runtime"
	"unsafe"
)

const (
	domainJobTimeElapsedField            = "time_elapsed"
	domainJobTimeElapsedNetField         = "time_elapsed_net"
	domainJobTimeRemainingField          = "time_remaining"
	domainJobDowntimeField               = "downtime"
	domainJobDowntimeNetField            = "downtime_net"
	domainJobSetupTimeField              = "setup_time"
	domainJobDataTotalField              = "data_total"
	domainJobDataProcessedField          = "data_processed"
	domainJobDataRemainingField          = "data_remaining"
	domainJobMemoryTotalField            = "memory_total"
	domainJobMemoryProcessedField        = "memory_processed"
	domainJobMemoryRemainingField        = "memory_remaining"
	domainJobMemoryConstantField         = "memory_constant"
	domainJobMemoryNormalField           = "memory_normal"
	domainJobMemoryNormalBytesField      = "memory_normal_bytes"
	domainJobMemoryBPSField              = "memory_bps"
	domainJobMemoryDirtyRateField        = "memory_dirty_rate"
	domainJobMemoryPageSizeField         = "memory_page_size"
	domainJobMemoryIterationField        = "memory_iteration"
	domainJobDiskTotalField              = "disk_total"
	domainJobDiskProcessedField          = "disk_processed"
	domainJobDiskRemainingField          = "disk_remaining"
	domainJobDiskBPSField                = "disk_bps"
	domainJobCompressionCacheField       = "compression_cache"
	domainJobCompressionBytesField       = "compression_bytes"
	domainJobCompressionPagesField       = "compression_pages"
	domainJobCompressionCacheMissesField = "compression_cache_misses"
	domainJobCompressionOverflowField    = "compression_overflow"
	domainJobAutoConvergeThrottleField   = "auto_converge_throttle"
	domainJobOperationField              = "operation"
	domainJobMemoryPostcopyRequestsField = "memory_postcopy_requests"
	domainJobSuccessField                = "success"
	domainJobDiskTemporaryUsedField      = "disk_temp_used"
	domainJobDiskTemporaryTotalField     = "disk_temp_total"
	domainJobErrorField                  = "errmsg"
)

// DomainJobType describes the lifecycle state of a libvirt domain job.
type DomainJobType int32

const (
	DomainJobNone      DomainJobType = VIR_DOMAIN_JOB_NONE
	DomainJobBounded   DomainJobType = VIR_DOMAIN_JOB_BOUNDED
	DomainJobUnbounded DomainJobType = VIR_DOMAIN_JOB_UNBOUNDED
	DomainJobCompleted DomainJobType = VIR_DOMAIN_JOB_COMPLETED
	DomainJobFailed    DomainJobType = VIR_DOMAIN_JOB_FAILED
	DomainJobCancelled DomainJobType = VIR_DOMAIN_JOB_CANCELLED
)

// DomainJobOperationType identifies the operation represented by job statistics.
type DomainJobOperationType int32

const (
	DomainJobOperationUnknown        DomainJobOperationType = VIR_DOMAIN_JOB_OPERATION_UNKNOWN
	DomainJobOperationStart          DomainJobOperationType = VIR_DOMAIN_JOB_OPERATION_START
	DomainJobOperationSave           DomainJobOperationType = VIR_DOMAIN_JOB_OPERATION_SAVE
	DomainJobOperationRestore        DomainJobOperationType = VIR_DOMAIN_JOB_OPERATION_RESTORE
	DomainJobOperationMigrationIn    DomainJobOperationType = VIR_DOMAIN_JOB_OPERATION_MIGRATION_IN
	DomainJobOperationMigrationOut   DomainJobOperationType = VIR_DOMAIN_JOB_OPERATION_MIGRATION_OUT
	DomainJobOperationSnapshot       DomainJobOperationType = VIR_DOMAIN_JOB_OPERATION_SNAPSHOT
	DomainJobOperationSnapshotRevert DomainJobOperationType = VIR_DOMAIN_JOB_OPERATION_SNAPSHOT_REVERT
	DomainJobOperationDump           DomainJobOperationType = VIR_DOMAIN_JOB_OPERATION_DUMP
	DomainJobOperationBackup         DomainJobOperationType = VIR_DOMAIN_JOB_OPERATION_BACKUP
	DomainJobOperationSnapshotDelete DomainJobOperationType = VIR_DOMAIN_JOB_OPERATION_SNAPSHOT_DELETE
)

// DomainJobInfo contains the job state and decoded typed parameters.
type DomainJobInfo struct {
	Type                      DomainJobType
	TimeElapsedSet            bool
	TimeElapsed               uint64
	TimeElapsedNetSet         bool
	TimeElapsedNet            uint64
	TimeRemainingSet          bool
	TimeRemaining             uint64
	DowntimeSet               bool
	Downtime                  uint64
	DowntimeNetSet            bool
	DowntimeNet               uint64
	SetupTimeSet              bool
	SetupTime                 uint64
	DataTotalSet              bool
	DataTotal                 uint64
	DataProcessedSet          bool
	DataProcessed             uint64
	DataRemainingSet          bool
	DataRemaining             uint64
	MemTotalSet               bool
	MemTotal                  uint64
	MemProcessedSet           bool
	MemProcessed              uint64
	MemRemainingSet           bool
	MemRemaining              uint64
	MemConstantSet            bool
	MemConstant               uint64
	MemNormalSet              bool
	MemNormal                 uint64
	MemNormalBytesSet         bool
	MemNormalBytes            uint64
	MemBpsSet                 bool
	MemBps                    uint64
	MemDirtyRateSet           bool
	MemDirtyRate              uint64
	MemPageSizeSet            bool
	MemPageSize               uint64
	MemIterationSet           bool
	MemIteration              uint64
	DiskTotalSet              bool
	DiskTotal                 uint64
	DiskProcessedSet          bool
	DiskProcessed             uint64
	DiskRemainingSet          bool
	DiskRemaining             uint64
	DiskBpsSet                bool
	DiskBps                   uint64
	CompressionCacheSet       bool
	CompressionCache          uint64
	CompressionBytesSet       bool
	CompressionBytes          uint64
	CompressionPagesSet       bool
	CompressionPages          uint64
	CompressionCacheMissesSet bool
	CompressionCacheMisses    uint64
	CompressionOverflowSet    bool
	CompressionOverflow       uint64
	AutoConvergeThrottleSet   bool
	AutoConvergeThrottle      int
	OperationSet              bool
	Operation                 DomainJobOperationType
	MemPostcopyReqsSet        bool
	MemPostcopyReqs           uint64
	JobSuccessSet             bool
	JobSuccess                bool
	DiskTempUsedSet           bool
	DiskTempUsed              uint64
	DiskTempTotalSet          bool
	DiskTempTotal             uint64
	ErrorMessageSet           bool
	ErrorMessage              string
	Parameters                []TypedParameter
}

// BackupBegin starts a domain backup job.
func (d *Domain) BackupBegin(backupXML, checkpointXML string, flags uint32) error {
	backupBuffer, backupPtr, err := makeCString("backup XML", backupXML, false)
	if err != nil {
		return err
	}
	checkpointBuffer, checkpointPtr, err := makeCString("checkpoint XML", checkpointXML, true)
	if err != nil {
		return err
	}
	_, err = domainCall(d, "virDomainBackupBegin", func(api *nativeAPI, ptr unsafe.Pointer) (int32, bool) {
		result := api.virDomainBackupBegin(ptr, backupPtr, checkpointPtr, flags)
		return result, result < 0
	})
	runtime.KeepAlive(backupBuffer)
	runtime.KeepAlive(checkpointBuffer)
	return err
}

// AbortJob aborts the currently running domain job.
func (d *Domain) AbortJob() error {
	return d.callStatus("virDomainAbortJob", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virDomainAbortJob(ptr)
	})
}

// GetJobStats returns the current or retained completed job statistics.
func (d *Domain) GetJobStats(flags DomainGetJobStatsFlags) (*DomainJobInfo, error) {
	var jobType int32
	var memory unsafe.Pointer
	var count int32
	_, err := domainCall(d, "virDomainGetJobStats", func(api *nativeAPI, ptr unsafe.Pointer) (int32, bool) {
		result := api.virDomainGetJobStats(ptr, &jobType, &memory, &count, flags)
		return result, result < 0
	})
	if err != nil {
		return nil, err
	}
	if count > 0 && memory == nil {
		return nil, fmt.Errorf("libvirt: virDomainGetJobStats returned %d parameters with nil storage", count)
	}
	parameters, decodeErr := decodeTypedParameters(memory, count)
	if memory != nil {
		d.api.virTypedParamsFree(memory, count)
	}
	if decodeErr != nil {
		return nil, decodeErr
	}
	stats := &DomainJobInfo{Type: DomainJobType(jobType), Parameters: parameters}
	for _, parameter := range parameters {
		if err := decodeDomainJobParameter(stats, parameter); err != nil {
			return nil, err
		}
	}
	return stats, nil
}

func decodeDomainJobParameter(stats *DomainJobInfo, parameter TypedParameter) error {
	var present *bool
	var destination *uint64
	switch parameter.Field {
	case domainJobTimeElapsedField:
		present, destination = &stats.TimeElapsedSet, &stats.TimeElapsed
	case domainJobTimeElapsedNetField:
		present, destination = &stats.TimeElapsedNetSet, &stats.TimeElapsedNet
	case domainJobTimeRemainingField:
		present, destination = &stats.TimeRemainingSet, &stats.TimeRemaining
	case domainJobDowntimeField:
		present, destination = &stats.DowntimeSet, &stats.Downtime
	case domainJobDowntimeNetField:
		present, destination = &stats.DowntimeNetSet, &stats.DowntimeNet
	case domainJobSetupTimeField:
		present, destination = &stats.SetupTimeSet, &stats.SetupTime
	case domainJobDataTotalField:
		present, destination = &stats.DataTotalSet, &stats.DataTotal
	case domainJobDataProcessedField:
		present, destination = &stats.DataProcessedSet, &stats.DataProcessed
	case domainJobDataRemainingField:
		present, destination = &stats.DataRemainingSet, &stats.DataRemaining
	case domainJobMemoryTotalField:
		present, destination = &stats.MemTotalSet, &stats.MemTotal
	case domainJobMemoryProcessedField:
		present, destination = &stats.MemProcessedSet, &stats.MemProcessed
	case domainJobMemoryRemainingField:
		present, destination = &stats.MemRemainingSet, &stats.MemRemaining
	case domainJobMemoryConstantField:
		present, destination = &stats.MemConstantSet, &stats.MemConstant
	case domainJobMemoryNormalField:
		present, destination = &stats.MemNormalSet, &stats.MemNormal
	case domainJobMemoryNormalBytesField:
		present, destination = &stats.MemNormalBytesSet, &stats.MemNormalBytes
	case domainJobMemoryBPSField:
		present, destination = &stats.MemBpsSet, &stats.MemBps
	case domainJobMemoryDirtyRateField:
		present, destination = &stats.MemDirtyRateSet, &stats.MemDirtyRate
	case domainJobMemoryPageSizeField:
		present, destination = &stats.MemPageSizeSet, &stats.MemPageSize
	case domainJobMemoryIterationField:
		present, destination = &stats.MemIterationSet, &stats.MemIteration
	case domainJobDiskTotalField:
		present, destination = &stats.DiskTotalSet, &stats.DiskTotal
	case domainJobDiskProcessedField:
		present, destination = &stats.DiskProcessedSet, &stats.DiskProcessed
	case domainJobDiskRemainingField:
		present, destination = &stats.DiskRemainingSet, &stats.DiskRemaining
	case domainJobDiskBPSField:
		present, destination = &stats.DiskBpsSet, &stats.DiskBps
	case domainJobCompressionCacheField:
		present, destination = &stats.CompressionCacheSet, &stats.CompressionCache
	case domainJobCompressionBytesField:
		present, destination = &stats.CompressionBytesSet, &stats.CompressionBytes
	case domainJobCompressionPagesField:
		present, destination = &stats.CompressionPagesSet, &stats.CompressionPages
	case domainJobCompressionCacheMissesField:
		present, destination = &stats.CompressionCacheMissesSet, &stats.CompressionCacheMisses
	case domainJobCompressionOverflowField:
		present, destination = &stats.CompressionOverflowSet, &stats.CompressionOverflow
	case domainJobMemoryPostcopyRequestsField:
		present, destination = &stats.MemPostcopyReqsSet, &stats.MemPostcopyReqs
	case domainJobDiskTemporaryUsedField:
		present, destination = &stats.DiskTempUsedSet, &stats.DiskTempUsed
	case domainJobDiskTemporaryTotalField:
		present, destination = &stats.DiskTempTotalSet, &stats.DiskTempTotal
	}
	if destination != nil {
		value, ok := parameter.Value.(uint64)
		if !ok {
			return fmt.Errorf("libvirt: job field %q has type %T", parameter.Field, parameter.Value)
		}
		*present = true
		*destination = value
		return nil
	}

	switch parameter.Field {
	case domainJobAutoConvergeThrottleField:
		value, ok := parameter.Value.(int32)
		if !ok {
			return fmt.Errorf("libvirt: job field %q has type %T", parameter.Field, parameter.Value)
		}
		stats.AutoConvergeThrottleSet = true
		stats.AutoConvergeThrottle = int(value)
	case domainJobOperationField:
		value, ok := parameter.Value.(int32)
		if !ok {
			return fmt.Errorf("libvirt: job field %q has type %T", parameter.Field, parameter.Value)
		}
		stats.OperationSet = true
		stats.Operation = DomainJobOperationType(value)
	case domainJobSuccessField:
		value, ok := parameter.Value.(bool)
		if !ok {
			return fmt.Errorf("libvirt: job field %q has type %T", parameter.Field, parameter.Value)
		}
		stats.JobSuccessSet = true
		stats.JobSuccess = value
	case domainJobErrorField:
		value, ok := parameter.Value.(string)
		if !ok {
			return fmt.Errorf("libvirt: job field %q has type %T", parameter.Field, parameter.Value)
		}
		stats.ErrorMessageSet = true
		stats.ErrorMessage = value
	}
	return nil
}

// FSFreeze freezes guest filesystems mounted at the requested paths. An empty
// path list asks the guest agent to freeze every mounted filesystem.
func (d *Domain) FSFreeze(mountpoints []string, flags uint32) error {
	return d.filesystemCall("virDomainFSFreeze", mountpoints, flags, func(api *nativeAPI, ptr unsafe.Pointer, mounts *unsafe.Pointer, count uint32, flags uint32) int32 {
		return api.virDomainFSFreeze(ptr, mounts, count, flags)
	})
}

// FSThaw thaws guest filesystems mounted at the requested paths. An empty path
// list asks the guest agent to thaw every mounted filesystem.
func (d *Domain) FSThaw(mountpoints []string, flags uint32) error {
	return d.filesystemCall("virDomainFSThaw", mountpoints, flags, func(api *nativeAPI, ptr unsafe.Pointer, mounts *unsafe.Pointer, count uint32, flags uint32) int32 {
		return api.virDomainFSThaw(ptr, mounts, count, flags)
	})
}

func (d *Domain) filesystemCall(operation string, mountpoints []string, flags uint32, call func(*nativeAPI, unsafe.Pointer, *unsafe.Pointer, uint32, uint32) int32) error {
	if uint64(len(mountpoints)) > uint64(^uint32(0)) {
		return fmt.Errorf("libvirt: too many filesystem mountpoints")
	}
	buffers := make([][]byte, len(mountpoints))
	pointers := make([]unsafe.Pointer, len(mountpoints))
	for i, mountpoint := range mountpoints {
		buffer, pointer, err := makeCString("filesystem mountpoint", mountpoint, false)
		if err != nil {
			return err
		}
		buffers[i] = buffer
		pointers[i] = unsafe.Pointer(pointer)
	}
	var mounts *unsafe.Pointer
	if len(pointers) != 0 {
		mounts = &pointers[0]
	}
	_, err := domainCall(d, operation, func(api *nativeAPI, ptr unsafe.Pointer) (int32, bool) {
		result := call(api, ptr, mounts, uint32(len(pointers)), flags)
		return result, result < 0
	})
	runtime.KeepAlive(buffers)
	runtime.KeepAlive(pointers)
	return err
}
