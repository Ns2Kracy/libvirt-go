package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	libvirt "github.com/Ns2Kracy/libvirt-go"
)

type options struct {
	uri    string
	action string
	name   string
	xml    string
}

func main() {
	var opts options
	flag.StringVar(&opts.uri, "uri", "qemu:///session", "libvirt connection URI")
	flag.StringVar(&opts.action, "action", "list", "list, inspect, define, or undefine")
	flag.StringVar(&opts.name, "name", "", "network filter name")
	flag.StringVar(&opts.xml, "xml", "", "network filter XML file used by define")
	flag.Parse()

	if err := run(opts); err != nil {
		log.Fatal(err)
	}
}

func run(opts options) (err error) {
	opts.action = strings.ToLower(opts.action)
	var conn *libvirt.Connect
	if opts.action == "list" || opts.action == "inspect" {
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

	switch opts.action {
	case "list":
		return listFilters(conn)
	case "define":
		return defineFilter(conn, opts.xml)
	case "inspect", "undefine":
		if opts.name == "" {
			return errors.New("-name is required for this action")
		}
		return actOnFilter(conn, opts)
	default:
		return fmt.Errorf("unknown action %q", opts.action)
	}
}

func listFilters(conn *libvirt.Connect) (err error) {
	filters, err := conn.ListAllNWFilters(0)
	if err != nil {
		return fmt.Errorf("list network filters: %w", err)
	}
	defer func() {
		for _, filter := range filters {
			err = errors.Join(err, filter.Free())
		}
	}()
	for _, filter := range filters {
		name, err := filter.GetName()
		if err != nil {
			return fmt.Errorf("get network filter name: %w", err)
		}
		uuid, err := filter.GetUUIDString()
		if err != nil {
			return fmt.Errorf("get network filter %q UUID: %w", name, err)
		}
		fmt.Printf("%s uuid=%s\n", name, uuid)
	}
	return nil
}

func defineFilter(conn *libvirt.Connect, path string) (err error) {
	if path == "" {
		return errors.New("-xml is required for define")
	}
	document, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read network filter XML: %w", err)
	}
	filter, err := conn.NWFilterDefineXML(string(document))
	if err != nil {
		return fmt.Errorf("define network filter: %w", err)
	}
	defer func() { err = errors.Join(err, filter.Free()) }()

	name, err := filter.GetName()
	if err != nil {
		return fmt.Errorf("get defined network filter name: %w", err)
	}
	fmt.Println(name)
	return nil
}

func actOnFilter(conn *libvirt.Connect, opts options) (err error) {
	filter, err := conn.LookupNWFilterByName(opts.name)
	if err != nil {
		return fmt.Errorf("lookup network filter %q: %w", opts.name, err)
	}
	defer func() { err = errors.Join(err, filter.Free()) }()

	if opts.action == "undefine" {
		if err := filter.Undefine(); err != nil {
			return fmt.Errorf("undefine network filter %q: %w", opts.name, err)
		}
		fmt.Printf("undefined network filter %s\n", opts.name)
		return nil
	}
	document, err := filter.GetXMLDesc(0)
	if err != nil {
		return fmt.Errorf("get network filter %q XML: %w", opts.name, err)
	}
	fmt.Println(document)
	return nil
}
