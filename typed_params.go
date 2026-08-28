package libvirt

import (
	"bytes"
	"fmt"
	"math"
	"strings"
	"unsafe"
)

const typedParameterFieldLength = 80

// TypedParameterType identifies the value stored in a TypedParameter.
type TypedParameterType int32

const (
	TypedParameterInt       TypedParameterType = TypedParameterType(VIR_TYPED_PARAM_INT)
	TypedParameterUInt      TypedParameterType = TypedParameterType(VIR_TYPED_PARAM_UINT)
	TypedParameterLongLong  TypedParameterType = TypedParameterType(VIR_TYPED_PARAM_LLONG)
	TypedParameterULongLong TypedParameterType = TypedParameterType(VIR_TYPED_PARAM_ULLONG)
	TypedParameterDouble    TypedParameterType = TypedParameterType(VIR_TYPED_PARAM_DOUBLE)
	TypedParameterBoolean   TypedParameterType = TypedParameterType(VIR_TYPED_PARAM_BOOLEAN)
	TypedParameterString    TypedParameterType = TypedParameterType(VIR_TYPED_PARAM_STRING)
)

// TypedParameter is an ownership-safe Go representation of virTypedParameter.
// Value is one of int32, uint32, int64, uint64, float64, bool, or string.
type TypedParameter struct {
	Field string
	Type  TypedParameterType
	Value any
}

type cTypedParameterValue struct {
	bits uint64
}

type cTypedParameter struct {
	field [typedParameterFieldLength]byte
	type_ int32
	value cTypedParameterValue
}

func decodeTypedParameters(memory unsafe.Pointer, count int32) ([]TypedParameter, error) {
	if count == 0 {
		return []TypedParameter{}, nil
	}
	params := unsafe.Slice((*cTypedParameter)(memory), int(count))
	result := make([]TypedParameter, len(params))
	for i := range params {
		param := &params[i]
		end := bytes.IndexByte(param.field[:], 0)
		if end < 0 {
			end = len(param.field)
		}
		result[i].Field = string(param.field[:end])
		result[i].Type = TypedParameterType(param.type_)
		value := unsafe.Pointer(&param.value)
		switch result[i].Type {
		case TypedParameterInt:
			result[i].Value = *(*int32)(value)
		case TypedParameterUInt:
			result[i].Value = *(*uint32)(value)
		case TypedParameterLongLong:
			result[i].Value = *(*int64)(value)
		case TypedParameterULongLong:
			result[i].Value = *(*uint64)(value)
		case TypedParameterDouble:
			result[i].Value = *(*float64)(value)
		case TypedParameterBoolean:
			result[i].Value = *(*byte)(value) != 0
		case TypedParameterString:
			result[i].Value = copyCString(uintptrPointer(value))
		default:
			return nil, fmt.Errorf("libvirt: unknown typed parameter type %d for %q", param.type_, result[i].Field)
		}
	}
	return result, nil
}

func uintptrPointer(value unsafe.Pointer) unsafe.Pointer {
	return *(*unsafe.Pointer)(value)
}

func encodeTypedParameters(api *nativeAPI, params []TypedParameter) (unsafe.Pointer, int32, func(), error) {
	if len(params) > math.MaxInt32 {
		return nil, 0, nil, fmt.Errorf("libvirt: too many typed parameters: %d", len(params))
	}
	if len(params) == 0 {
		return nil, 0, func() {}, nil
	}
	memory := api.calloc(uintptr(len(params)), unsafe.Sizeof(cTypedParameter{}))
	if memory == nil {
		return nil, 0, nil, fmt.Errorf("libvirt: allocate typed parameters")
	}
	cleanup := func() {
		freeTypedParameterStrings(api, memory, int32(len(params)))
		api.free(memory)
	}
	encoded := unsafe.Slice((*cTypedParameter)(memory), len(params))
	for i, param := range params {
		if param.Field == "" || len(param.Field) >= typedParameterFieldLength || strings.IndexByte(param.Field, 0) >= 0 {
			cleanup()
			return nil, 0, nil, fmt.Errorf("libvirt: invalid typed parameter field %q", param.Field)
		}
		copy(encoded[i].field[:], param.Field)
		value := unsafe.Pointer(&encoded[i].value)
		typeTag, err := encodeTypedParameterValue(api, value, param)
		if err != nil {
			cleanup()
			return nil, 0, nil, err
		}
		encoded[i].type_ = int32(typeTag)
	}
	return memory, int32(len(params)), cleanup, nil
}

func encodeTypedParameterValue(api *nativeAPI, destination unsafe.Pointer, param TypedParameter) (TypedParameterType, error) {
	typeTag := param.Type
	if typeTag == 0 {
		switch param.Value.(type) {
		case int, int32:
			typeTag = TypedParameterInt
		case uint, uint32:
			typeTag = TypedParameterUInt
		case int64:
			typeTag = TypedParameterLongLong
		case uint64:
			typeTag = TypedParameterULongLong
		case float64:
			typeTag = TypedParameterDouble
		case bool:
			typeTag = TypedParameterBoolean
		case string:
			typeTag = TypedParameterString
		}
	}
	switch typeTag {
	case TypedParameterInt:
		var value int64
		switch typed := param.Value.(type) {
		case int:
			value = int64(typed)
		case int32:
			value = int64(typed)
		default:
			return 0, typedParameterValueError(param, "int or int32")
		}
		if value < math.MinInt32 || value > math.MaxInt32 {
			return 0, fmt.Errorf("libvirt: typed parameter %q overflows int32", param.Field)
		}
		*(*int32)(destination) = int32(value)
	case TypedParameterUInt:
		var value uint64
		switch typed := param.Value.(type) {
		case uint:
			value = uint64(typed)
		case uint32:
			value = uint64(typed)
		default:
			return 0, typedParameterValueError(param, "uint or uint32")
		}
		if value > math.MaxUint32 {
			return 0, fmt.Errorf("libvirt: typed parameter %q overflows uint32", param.Field)
		}
		*(*uint32)(destination) = uint32(value)
	case TypedParameterLongLong:
		value, ok := param.Value.(int64)
		if !ok {
			return 0, typedParameterValueError(param, "int64")
		}
		*(*int64)(destination) = value
	case TypedParameterULongLong:
		value, ok := param.Value.(uint64)
		if !ok {
			return 0, typedParameterValueError(param, "uint64")
		}
		*(*uint64)(destination) = value
	case TypedParameterDouble:
		value, ok := param.Value.(float64)
		if !ok {
			return 0, typedParameterValueError(param, "float64")
		}
		*(*float64)(destination) = value
	case TypedParameterBoolean:
		value, ok := param.Value.(bool)
		if !ok {
			return 0, typedParameterValueError(param, "bool")
		}
		if value {
			*(*byte)(destination) = 1
		}
	case TypedParameterString:
		value, ok := param.Value.(string)
		if !ok || strings.IndexByte(value, 0) >= 0 {
			return 0, typedParameterValueError(param, "NUL-free string")
		}
		allocated := api.malloc(uintptr(len(value) + 1))
		if allocated == nil {
			return 0, fmt.Errorf("libvirt: allocate typed parameter %q", param.Field)
		}
		buffer := unsafe.Slice((*byte)(allocated), len(value)+1)
		copy(buffer, value)
		buffer[len(value)] = 0
		*(*unsafe.Pointer)(destination) = allocated
	default:
		return 0, fmt.Errorf("libvirt: unsupported typed parameter type %d", typeTag)
	}
	return typeTag, nil
}

func typedParameterValueError(param TypedParameter, expected string) error {
	return fmt.Errorf("libvirt: typed parameter %q expects %s, got %T", param.Field, expected, param.Value)
}

func freeTypedParameterStrings(api *nativeAPI, memory unsafe.Pointer, count int32) {
	if memory == nil || count <= 0 {
		return
	}
	params := unsafe.Slice((*cTypedParameter)(memory), int(count))
	for i := range params {
		if TypedParameterType(params[i].type_) == TypedParameterString {
			value := uintptrPointer(unsafe.Pointer(&params[i].value))
			if value != nil {
				api.free(value)
			}
		}
	}
}

func getDomainTypedParameters(domain *Domain, operation string, flags uint32, get func(*nativeAPI, unsafe.Pointer, unsafe.Pointer, *int32, uint32) int32) ([]TypedParameter, error) {
	var count int32
	_, err := domainCall(domain, operation, func(api *nativeAPI, ptr unsafe.Pointer) (int32, bool) {
		result := get(api, ptr, nil, &count, flags)
		return result, result < 0
	})
	if err != nil || count == 0 {
		return []TypedParameter{}, err
	}
	api := domain.api
	memory := api.calloc(uintptr(count), unsafe.Sizeof(cTypedParameter{}))
	if memory == nil {
		return nil, fmt.Errorf("libvirt: allocate %d typed parameters", count)
	}
	allocatedCount := count
	defer func() {
		freeTypedParameterStrings(api, memory, allocatedCount)
		api.free(memory)
	}()
	_, err = domainCall(domain, operation, func(api *nativeAPI, ptr unsafe.Pointer) (int32, bool) {
		result := get(api, ptr, memory, &count, flags)
		return result, result < 0
	})
	if err != nil {
		return nil, err
	}
	return decodeTypedParameters(memory, count)
}

func setDomainTypedParameters(domain *Domain, operation string, params []TypedParameter, flags uint32, set func(*nativeAPI, unsafe.Pointer, unsafe.Pointer, int32, uint32) int32) error {
	if domain == nil {
		return fmt.Errorf("%w: domain", ErrClosed)
	}
	memory, count, cleanup, err := encodeTypedParameters(domain.api, params)
	if err != nil {
		return err
	}
	defer cleanup()
	_, err = domainCall(domain, operation, func(api *nativeAPI, ptr unsafe.Pointer) (int32, bool) {
		result := set(api, ptr, memory, count, flags)
		return result, result < 0
	})
	return err
}

// GetMemoryParameters returns domain memory tuning parameters.
func (domain *Domain) GetMemoryParameters(flags uint32) ([]TypedParameter, error) {
	return getDomainTypedParameters(domain, "virDomainGetMemoryParameters", flags, func(api *nativeAPI, ptr unsafe.Pointer, params unsafe.Pointer, count *int32, flags uint32) int32 {
		return api.virDomainGetMemoryParameters(ptr, params, count, flags)
	})
}

// SetMemoryParameters updates domain memory tuning parameters.
func (domain *Domain) SetMemoryParameters(params []TypedParameter, flags uint32) error {
	return setDomainTypedParameters(domain, "virDomainSetMemoryParameters", params, flags, func(api *nativeAPI, ptr unsafe.Pointer, params unsafe.Pointer, count int32, flags uint32) int32 {
		return api.virDomainSetMemoryParameters(ptr, params, count, flags)
	})
}

// GetNumaParameters returns domain NUMA tuning parameters.
func (domain *Domain) GetNumaParameters(flags uint32) ([]TypedParameter, error) {
	return getDomainTypedParameters(domain, "virDomainGetNumaParameters", flags, func(api *nativeAPI, ptr unsafe.Pointer, params unsafe.Pointer, count *int32, flags uint32) int32 {
		return api.virDomainGetNumaParameters(ptr, params, count, flags)
	})
}

// SetNumaParameters updates domain NUMA tuning parameters.
func (domain *Domain) SetNumaParameters(params []TypedParameter, flags uint32) error {
	return setDomainTypedParameters(domain, "virDomainSetNumaParameters", params, flags, func(api *nativeAPI, ptr unsafe.Pointer, params unsafe.Pointer, count int32, flags uint32) int32 {
		return api.virDomainSetNumaParameters(ptr, params, count, flags)
	})
}

// GetSchedulerParameters returns domain scheduler parameters.
func (domain *Domain) GetSchedulerParameters(flags uint32) ([]TypedParameter, error) {
	return getDomainTypedParameters(domain, "virDomainGetSchedulerParametersFlags", flags, func(api *nativeAPI, ptr unsafe.Pointer, params unsafe.Pointer, count *int32, flags uint32) int32 {
		return api.virDomainGetSchedulerParametersFlags(ptr, params, count, flags)
	})
}

// SetSchedulerParameters updates domain scheduler parameters.
func (domain *Domain) SetSchedulerParameters(params []TypedParameter, flags uint32) error {
	return setDomainTypedParameters(domain, "virDomainSetSchedulerParametersFlags", params, flags, func(api *nativeAPI, ptr unsafe.Pointer, params unsafe.Pointer, count int32, flags uint32) int32 {
		return api.virDomainSetSchedulerParametersFlags(ptr, params, count, flags)
	})
}

// GetBlockIOParameters returns domain block-I/O tuning parameters.
func (domain *Domain) GetBlockIOParameters(flags uint32) ([]TypedParameter, error) {
	return getDomainTypedParameters(domain, "virDomainGetBlkioParameters", flags, func(api *nativeAPI, ptr unsafe.Pointer, params unsafe.Pointer, count *int32, flags uint32) int32 {
		return api.virDomainGetBlkioParameters(ptr, params, count, flags)
	})
}

// SetBlockIOParameters updates domain block-I/O tuning parameters.
func (domain *Domain) SetBlockIOParameters(params []TypedParameter, flags uint32) error {
	return setDomainTypedParameters(domain, "virDomainSetBlkioParameters", params, flags, func(api *nativeAPI, ptr unsafe.Pointer, params unsafe.Pointer, count int32, flags uint32) int32 {
		return api.virDomainSetBlkioParameters(ptr, params, count, flags)
	})
}
