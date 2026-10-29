package libvirt

import (
	"bytes"
	"fmt"
	"math"
	"runtime"
	"strings"
	"unsafe"
)

const typedParameterFieldLength = TYPED_PARAM_FIELD_LENGTH

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

func decodeTypedParameters(memory unsafe.Pointer, count int32) ([]TypedParameter, error) {
	if count < 0 {
		return nil, fmt.Errorf("libvirt: negative typed parameter count %d", count)
	}
	params := unsafe.Slice((*cTypedParameter)(memory), int(count))
	result := make([]TypedParameter, len(params))
	for i := range params {
		param, err := decodeTypedParameter(&params[i])
		if err != nil {
			return nil, err
		}
		result[i] = param
	}
	return result, nil
}

// decodeTypedParameter copies a native parameter, including strings, into Go-owned storage.
func decodeTypedParameter(param *cTypedParameter) (TypedParameter, error) {
	end := bytes.IndexByte(param.field[:], 0)
	if end < 0 {
		end = len(param.field)
	}
	result := TypedParameter{Field: string(param.field[:end]), Type: TypedParameterType(param.type_)}
	value := unsafe.Pointer(&param.value)
	switch result.Type {
	case TypedParameterInt:
		result.Value = *(*int32)(value)
	case TypedParameterUInt:
		result.Value = *(*uint32)(value)
	case TypedParameterLongLong:
		result.Value = *(*int64)(value)
	case TypedParameterULongLong:
		result.Value = *(*uint64)(value)
	case TypedParameterDouble:
		result.Value = *(*float64)(value)
	case TypedParameterBoolean:
		result.Value = *(*byte)(value) != 0
	case TypedParameterString:
		result.Value = copyCString(uintptrPointer(value))
	default:
		return TypedParameter{}, fmt.Errorf("libvirt: unknown typed parameter type %d for %q", param.type_, result.Field)
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
	normalized := make([]TypedParameter, len(params))
	for i, param := range params {
		value, err := normalizeTypedParameter(param)
		if err != nil {
			return nil, 0, nil, err
		}
		normalized[i] = value
	}
	var memory unsafe.Pointer
	var count, capacity int32
	// These helpers were introduced together in 1.0.2. Retain the original ABI
	// marshaller when an older or partially backported library lacks any of them.
	native := api.virTypedParamsFree != nil && api.virTypedParamsAddInt != nil &&
		api.virTypedParamsAddUInt != nil && api.virTypedParamsAddLLong != nil &&
		api.virTypedParamsAddULLong != nil && api.virTypedParamsAddDouble != nil &&
		api.virTypedParamsAddBoolean != nil && api.virTypedParamsAddString != nil
	cleanup := func() {
		if native {
			api.virTypedParamsFree(memory, count)
		} else {
			freeTypedParameterStrings(api, memory, count)
			api.free(memory)
		}
	}
	if len(params) == 0 {
		return nil, 0, func() {}, nil
	}
	if !native {
		memory = api.calloc(uintptr(len(params)), unsafe.Sizeof(cTypedParameter{}))
		if memory == nil {
			return nil, 0, nil, fmt.Errorf("libvirt: allocate typed parameters")
		}
		count = int32(len(params))
	}
	for i, param := range normalized {
		if native {
			name := append([]byte(param.Field), 0)
			var add func() int32
			var operation string
			var text []byte
			switch value := param.Value.(type) {
			case int32:
				operation = "virTypedParamsAddInt"
				add = func() int32 { return api.virTypedParamsAddInt(&memory, &count, &capacity, &name[0], value) }
			case uint32:
				operation = "virTypedParamsAddUInt"
				add = func() int32 { return api.virTypedParamsAddUInt(&memory, &count, &capacity, &name[0], value) }
			case int64:
				operation = "virTypedParamsAddLLong"
				add = func() int32 { return api.virTypedParamsAddLLong(&memory, &count, &capacity, &name[0], value) }
			case uint64:
				operation = "virTypedParamsAddULLong"
				add = func() int32 { return api.virTypedParamsAddULLong(&memory, &count, &capacity, &name[0], value) }
			case float64:
				operation = "virTypedParamsAddDouble"
				add = func() int32 { return api.virTypedParamsAddDouble(&memory, &count, &capacity, &name[0], value) }
			case bool:
				var boolean int32
				if value {
					boolean = 1
				}
				operation = "virTypedParamsAddBoolean"
				add = func() int32 { return api.virTypedParamsAddBoolean(&memory, &count, &capacity, &name[0], boolean) }
			case string:
				text = append([]byte(value), 0)
				operation = "virTypedParamsAddString"
				add = func() int32 { return api.virTypedParamsAddString(&memory, &count, &capacity, &name[0], &text[0]) }
			}
			_, err := nativeCall(api, operation, func() (int32, bool) { value := add(); return value, value < 0 })
			runtime.KeepAlive(name)
			runtime.KeepAlive(text)
			if err != nil {
				cleanup()
				return nil, 0, nil, err
			}
			continue
		}
		encoded := &unsafe.Slice((*cTypedParameter)(memory), len(params))[i]
		copy(encoded.field[:], param.Field)
		encoded.type_ = int32(param.Type)
		destination := unsafe.Pointer(&encoded.value)
		switch value := param.Value.(type) {
		case int32:
			*(*int32)(destination) = value
		case uint32:
			*(*uint32)(destination) = value
		case int64:
			*(*int64)(destination) = value
		case uint64:
			*(*uint64)(destination) = value
		case float64:
			*(*float64)(destination) = value
		case bool:
			if value {
				*(*byte)(destination) = 1
			}
		case string:
			allocated := api.malloc(uintptr(len(value) + 1))
			if allocated == nil {
				cleanup()
				return nil, 0, nil, fmt.Errorf("libvirt: allocate typed parameter %q", param.Field)
			}
			copy(unsafe.Slice((*byte)(allocated), len(value)+1), append([]byte(value), 0))
			*(*unsafe.Pointer)(destination) = allocated
		}
	}
	return memory, count, cleanup, nil
}

func normalizeTypedParameter(param TypedParameter) (TypedParameter, error) {
	if param.Field == "" || len(param.Field) >= typedParameterFieldLength || strings.IndexByte(param.Field, 0) >= 0 {
		return TypedParameter{}, fmt.Errorf("libvirt: invalid typed parameter field %q", param.Field)
	}
	if param.Type == 0 {
		switch param.Value.(type) {
		case int, int32:
			param.Type = TypedParameterInt
		case uint, uint32:
			param.Type = TypedParameterUInt
		case int64:
			param.Type = TypedParameterLongLong
		case uint64:
			param.Type = TypedParameterULongLong
		case float64:
			param.Type = TypedParameterDouble
		case bool:
			param.Type = TypedParameterBoolean
		case string:
			param.Type = TypedParameterString
		}
	}
	switch param.Type {
	case TypedParameterInt:
		var value int64
		switch typed := param.Value.(type) {
		case int:
			value = int64(typed)
		case int32:
			value = int64(typed)
		default:
			return TypedParameter{}, typedParameterValueError(param, "int or int32")
		}
		if value < math.MinInt32 || value > math.MaxInt32 {
			return TypedParameter{}, fmt.Errorf("libvirt: typed parameter %q overflows int32", param.Field)
		}
		param.Value = int32(value)
	case TypedParameterUInt:
		var value uint64
		switch typed := param.Value.(type) {
		case uint:
			value = uint64(typed)
		case uint32:
			value = uint64(typed)
		default:
			return TypedParameter{}, typedParameterValueError(param, "uint or uint32")
		}
		if value > math.MaxUint32 {
			return TypedParameter{}, fmt.Errorf("libvirt: typed parameter %q overflows uint32", param.Field)
		}
		param.Value = uint32(value)
	case TypedParameterLongLong:
		if _, ok := param.Value.(int64); !ok {
			return TypedParameter{}, typedParameterValueError(param, "int64")
		}
	case TypedParameterULongLong:
		if _, ok := param.Value.(uint64); !ok {
			return TypedParameter{}, typedParameterValueError(param, "uint64")
		}
	case TypedParameterDouble:
		if _, ok := param.Value.(float64); !ok {
			return TypedParameter{}, typedParameterValueError(param, "float64")
		}
	case TypedParameterBoolean:
		if _, ok := param.Value.(bool); !ok {
			return TypedParameter{}, typedParameterValueError(param, "bool")
		}
	case TypedParameterString:
		value, ok := param.Value.(string)
		if !ok || strings.IndexByte(value, 0) >= 0 {
			return TypedParameter{}, typedParameterValueError(param, "NUL-free string")
		}
	default:
		return TypedParameter{}, fmt.Errorf("libvirt: unsupported typed parameter type %d", param.Type)
	}
	return param, nil
}

func typedParameterValueError(param TypedParameter, expected string) error {
	return fmt.Errorf("libvirt: typed parameter %q expects %s, got %T", param.Field, expected, param.Value)
}

func freeTypedParameterStrings(api *nativeAPI, memory unsafe.Pointer, count int32) {
	if memory == nil || count <= 0 {
		return
	}
	if api.virTypedParamsClear != nil {
		api.virTypedParamsClear(memory, count)
		return
	}
	// Compatibility for libraries older than libvirt 1.0.2.
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

func getDomainTypedParameters(domain *Domain, operation string, count int32, flags uint32, get func(*nativeAPI, unsafe.Pointer, unsafe.Pointer, *int32, uint32) int32) ([]TypedParameter, error) {
	if count == 0 {
		_, err := domainCall(domain, operation, func(api *nativeAPI, ptr unsafe.Pointer) (int32, bool) {
			result := get(api, ptr, nil, &count, flags)
			return result, result < 0
		})
		if err != nil || count == 0 {
			return []TypedParameter{}, err
		}
	}
	if count < 0 {
		return nil, fmt.Errorf("libvirt: negative typed parameter count %d", count)
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
	_, err := domainCall(domain, operation, func(api *nativeAPI, ptr unsafe.Pointer) (int32, bool) {
		result := get(api, ptr, memory, &count, flags)
		return result, result < 0
	})
	if err != nil {
		return nil, err
	}
	if count < 0 || count > allocatedCount {
		return nil, fmt.Errorf("libvirt: typed parameter count changed from %d to %d", allocatedCount, count)
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
	return getDomainTypedParameters(domain, "virDomainGetMemoryParameters", 0, flags, func(api *nativeAPI, ptr unsafe.Pointer, params unsafe.Pointer, count *int32, flags uint32) int32 {
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
	return getDomainTypedParameters(domain, "virDomainGetNumaParameters", 0, flags, func(api *nativeAPI, ptr unsafe.Pointer, params unsafe.Pointer, count *int32, flags uint32) int32 {
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
	count, err := domainCall(domain, "virDomainGetSchedulerType", func(api *nativeAPI, ptr unsafe.Pointer) (int32, bool) {
		var count int32
		name := api.virDomainGetSchedulerType(ptr, &count)
		if name == nil {
			return 0, true
		}
		api.free(name)
		return count, false
	})
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return []TypedParameter{}, nil
	}
	return getDomainTypedParameters(domain, "virDomainGetSchedulerParametersFlags", count, flags, func(api *nativeAPI, ptr unsafe.Pointer, params unsafe.Pointer, count *int32, flags uint32) int32 {
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
	return getDomainTypedParameters(domain, "virDomainGetBlkioParameters", 0, flags, func(api *nativeAPI, ptr unsafe.Pointer, params unsafe.Pointer, count *int32, flags uint32) int32 {
		return api.virDomainGetBlkioParameters(ptr, params, count, flags)
	})
}

// SetBlockIOParameters updates domain block-I/O tuning parameters.
func (domain *Domain) SetBlockIOParameters(params []TypedParameter, flags uint32) error {
	return setDomainTypedParameters(domain, "virDomainSetBlkioParameters", params, flags, func(api *nativeAPI, ptr unsafe.Pointer, params unsafe.Pointer, count int32, flags uint32) int32 {
		return api.virDomainSetBlkioParameters(ptr, params, count, flags)
	})
}
