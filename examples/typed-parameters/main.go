package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"

	libvirt "github.com/Ns2Kracy/libvirt-go"
)

type options struct {
	uri       string
	domain    string
	group     string
	action    string
	field     string
	valueType string
	value     string
	flags     uint
}

type getParameters func(uint32) ([]libvirt.TypedParameter, error)
type setParameters func([]libvirt.TypedParameter, uint32) error

func main() {
	var opts options
	flag.StringVar(&opts.uri, "uri", "test:///default", "libvirt connection URI")
	flag.StringVar(&opts.domain, "domain", "test", "domain name")
	flag.StringVar(&opts.group, "group", "memory", "memory, numa, scheduler, or blockio")
	flag.StringVar(&opts.action, "action", "get", "get or set")
	flag.StringVar(&opts.field, "field", "", "parameter field used by set")
	flag.StringVar(&opts.valueType, "type", "ulonglong", "int, uint, longlong, ulonglong, double, bool, or string")
	flag.StringVar(&opts.value, "value", "", "parameter value used by set")
	flag.UintVar(&opts.flags, "flags", 0, "libvirt operation flags as an unsigned integer")
	flag.Parse()

	if err := run(opts); err != nil {
		log.Fatal(err)
	}
}

func run(opts options) (err error) {
	opts.group = strings.ToLower(opts.group)
	opts.action = strings.ToLower(opts.action)
	if opts.flags > math.MaxUint32 {
		return fmt.Errorf("flags value %d exceeds uint32", opts.flags)
	}
	if opts.action != "get" && opts.action != "set" {
		return fmt.Errorf("unknown action %q", opts.action)
	}

	var conn *libvirt.Connect
	if opts.action == "get" {
		conn, err = libvirt.NewConnectReadOnly(opts.uri)
	} else {
		conn, err = libvirt.NewConnect(opts.uri)
	}
	if err != nil {
		return fmt.Errorf("open %q: %w", opts.uri, err)
	}
	defer func() {
		_, closeErr := conn.Close()
		err = errors.Join(err, closeErr)
	}()

	domain, err := conn.LookupDomainByName(opts.domain)
	if err != nil {
		return fmt.Errorf("lookup domain %q: %w", opts.domain, err)
	}
	defer func() { err = errors.Join(err, domain.Free()) }()

	get, set, err := parameterMethods(domain, opts.group)
	if err != nil {
		return err
	}
	flags := uint32(opts.flags)
	if opts.action == "set" {
		parameter, err := parseParameter(opts)
		if err != nil {
			return err
		}
		if err := set([]libvirt.TypedParameter{parameter}, flags); err != nil {
			return fmt.Errorf("set %s parameter %q: %w", opts.group, opts.field, err)
		}
	}

	parameters, err := get(flags)
	if err != nil {
		return fmt.Errorf("get %s parameters: %w", opts.group, err)
	}
	for _, parameter := range parameters {
		fmt.Printf("%s type=%s value=%v\n", parameter.Field, parameterTypeName(parameter.Type), parameter.Value)
	}
	return nil
}

func parameterMethods(domain *libvirt.Domain, group string) (getParameters, setParameters, error) {
	switch group {
	case "memory":
		return domain.GetMemoryParameters, domain.SetMemoryParameters, nil
	case "numa":
		return domain.GetNumaParameters, domain.SetNumaParameters, nil
	case "scheduler":
		return domain.GetSchedulerParameters, domain.SetSchedulerParameters, nil
	case "blockio":
		return domain.GetBlockIOParameters, domain.SetBlockIOParameters, nil
	default:
		return nil, nil, fmt.Errorf("unknown parameter group %q", group)
	}
}

func parseParameter(opts options) (libvirt.TypedParameter, error) {
	if opts.field == "" {
		return libvirt.TypedParameter{}, errors.New("-field is required for set")
	}

	parameter := libvirt.TypedParameter{Field: opts.field}
	var err error
	switch strings.ToLower(opts.valueType) {
	case "int":
		parameter.Type = libvirt.TypedParameterInt
		var value int64
		value, err = strconv.ParseInt(opts.value, 0, 32)
		parameter.Value = int32(value)
	case "uint":
		parameter.Type = libvirt.TypedParameterUInt
		var value uint64
		value, err = strconv.ParseUint(opts.value, 0, 32)
		parameter.Value = uint32(value)
	case "longlong":
		parameter.Type = libvirt.TypedParameterLongLong
		parameter.Value, err = strconv.ParseInt(opts.value, 0, 64)
	case "ulonglong":
		parameter.Type = libvirt.TypedParameterULongLong
		parameter.Value, err = strconv.ParseUint(opts.value, 0, 64)
	case "double":
		parameter.Type = libvirt.TypedParameterDouble
		parameter.Value, err = strconv.ParseFloat(opts.value, 64)
	case "bool":
		parameter.Type = libvirt.TypedParameterBoolean
		parameter.Value, err = strconv.ParseBool(opts.value)
	case "string":
		parameter.Type = libvirt.TypedParameterString
		parameter.Value = opts.value
	default:
		return libvirt.TypedParameter{}, fmt.Errorf("unknown parameter type %q", opts.valueType)
	}
	if err != nil {
		return libvirt.TypedParameter{}, fmt.Errorf("parse %s value %q: %w", opts.valueType, opts.value, err)
	}
	return parameter, nil
}

func parameterTypeName(parameterType libvirt.TypedParameterType) string {
	switch parameterType {
	case libvirt.TypedParameterInt:
		return "int"
	case libvirt.TypedParameterUInt:
		return "uint"
	case libvirt.TypedParameterLongLong:
		return "longlong"
	case libvirt.TypedParameterULongLong:
		return "ulonglong"
	case libvirt.TypedParameterDouble:
		return "double"
	case libvirt.TypedParameterBoolean:
		return "bool"
	case libvirt.TypedParameterString:
		return "string"
	default:
		return fmt.Sprintf("unknown(%d)", parameterType)
	}
}
