package libvirt

import (
	"math"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestIntegrationTypedParameterOwnershipAndCompatibility(t *testing.T) {
	if os.Getenv("LIBVIRT_INTEGRATION") != "1" {
		t.Skip("set LIBVIRT_INTEGRATION=1 for native allocation round trips")
	}
	api, err := getNativeAPI()
	if err != nil {
		t.Fatal(err)
	}
	input := []TypedParameter{
		{Field: "int", Value: int(math.MinInt32)},
		{Field: "uint", Value: uint(math.MaxUint32)},
		{Field: "llong", Value: int64(math.MinInt64)},
		{Field: "ullong", Value: uint64(math.MaxUint64)},
		{Field: "double", Value: math.Inf(1)},
		{Field: "true", Value: true},
		{Field: "false", Value: false},
		{Field: "string", Value: "owned UTF-8 文本"},
		{Field: "string", Value: "duplicate fields preserve order"},
	}
	for _, mode := range []string{"native", "partial helpers", "pre-1.0.2"} {
		t.Run(mode, func(t *testing.T) {
			compatible := *api
			if mode != "native" {
				compatible.virTypedParamsAddInt = nil
			}
			if mode == "pre-1.0.2" {
				compatible.virTypedParamsFree = nil
				compatible.virTypedParamsClear = nil
			}
			memory, count, cleanup, err := encodeTypedParameters(&compatible, input)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := decodeTypedParameters(memory, count)
			cleanup()
			if err != nil {
				t.Fatal(err)
			}
			if len(decoded) != len(input) {
				t.Fatalf("decoded %d parameters", len(decoded))
			}
			for i, param := range input {
				expected, err := normalizeTypedParameter(param)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(decoded[i], expected) {
					t.Errorf("parameter %d after native cleanup = %#v, want %#v", i, decoded[i], expected)
				}
			}
			for _, invalid := range []TypedParameter{
				{Field: "", Value: 1}, {Field: strings.Repeat("x", TYPED_PARAM_FIELD_LENGTH), Value: 1},
				{Field: "bad\x00name", Value: 1}, {Field: "string", Value: "bad\x00value"},
				{Field: "overflow", Value: int(math.MaxInt32) + 1}, {Field: "overflow", Value: uint(math.MaxUint32) + 1},
				{Field: "type", Type: TypedParameterUInt, Value: int64(1)},
			} {
				memory, _, _, err := encodeTypedParameters(&compatible, []TypedParameter{invalid})
				if err == nil || memory != nil {
					t.Fatalf("invalid parameter accepted: %#v", invalid)
				}
			}
		})
	}
}
