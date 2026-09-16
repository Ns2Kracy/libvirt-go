package libvirt

import (
	"fmt"
	"runtime"
	"unsafe"
)

const (
	domainJobOperationField = "operation"
	domainJobSuccessField   = "success"
	domainJobErrorField     = "errmsg"
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

// DomainJobOperation identifies the operation represented by job statistics.
type DomainJobOperation int32

const (
	DomainJobOperationUnknown        DomainJobOperation = VIR_DOMAIN_JOB_OPERATION_UNKNOWN
	DomainJobOperationStart          DomainJobOperation = VIR_DOMAIN_JOB_OPERATION_START
	DomainJobOperationSave           DomainJobOperation = VIR_DOMAIN_JOB_OPERATION_SAVE
	DomainJobOperationRestore        DomainJobOperation = VIR_DOMAIN_JOB_OPERATION_RESTORE
	DomainJobOperationMigrationIn    DomainJobOperation = VIR_DOMAIN_JOB_OPERATION_MIGRATION_IN
	DomainJobOperationMigrationOut   DomainJobOperation = VIR_DOMAIN_JOB_OPERATION_MIGRATION_OUT
	DomainJobOperationSnapshot       DomainJobOperation = VIR_DOMAIN_JOB_OPERATION_SNAPSHOT
	DomainJobOperationSnapshotRevert DomainJobOperation = VIR_DOMAIN_JOB_OPERATION_SNAPSHOT_REVERT
	DomainJobOperationDump           DomainJobOperation = VIR_DOMAIN_JOB_OPERATION_DUMP
	DomainJobOperationBackup         DomainJobOperation = VIR_DOMAIN_JOB_OPERATION_BACKUP
	DomainJobOperationSnapshotDelete DomainJobOperation = VIR_DOMAIN_JOB_OPERATION_SNAPSHOT_DELETE
)

// DomainJobStats contains the job state and decoded typed parameters.
type DomainJobStats struct {
	Type         DomainJobType
	Operation    *DomainJobOperation
	Success      *bool
	ErrorMessage string
	Parameters   []TypedParameter
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
func (d *Domain) GetJobStats(flags uint32) (*DomainJobStats, error) {
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
	stats := &DomainJobStats{Type: DomainJobType(jobType), Parameters: parameters}
	for _, parameter := range parameters {
		switch parameter.Field {
		case domainJobOperationField:
			operation, ok := parameter.Value.(int32)
			if !ok {
				return nil, fmt.Errorf("libvirt: job operation has type %T", parameter.Value)
			}
			stats.Operation = new(DomainJobOperation(operation))
		case domainJobSuccessField:
			success, ok := parameter.Value.(bool)
			if !ok {
				return nil, fmt.Errorf("libvirt: job success has type %T", parameter.Value)
			}
			stats.Success = new(success)
		case domainJobErrorField:
			message, ok := parameter.Value.(string)
			if !ok {
				return nil, fmt.Errorf("libvirt: job error message has type %T", parameter.Value)
			}
			stats.ErrorMessage = message
		}
	}
	return stats, nil
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
