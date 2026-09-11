package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"strings"

	libvirt "github.com/Ns2Kracy/libvirt-go"
)

type options struct {
	uri    string
	action string
	name   string
	xml    string
	flags  uint
}

func main() {
	var opts options
	flag.StringVar(&opts.uri, "uri", "test:///default", "libvirt connection URI")
	flag.StringVar(&opts.action, "action", "list", "list, inspect, define, start, shutdown, destroy, or undefine")
	flag.StringVar(&opts.name, "name", "", "domain name (required except for list and define)")
	flag.StringVar(&opts.xml, "xml", "", "domain XML file (required for define)")
	flag.UintVar(&opts.flags, "flags", 0, "undefine flags as an unsigned integer")
	flag.Parse()

	if err := run(opts); err != nil {
		log.Fatal(err)
	}
}

func run(opts options) (err error) {
	opts.action = strings.ToLower(opts.action)
	if opts.flags > math.MaxUint32 {
		return fmt.Errorf("flags value %d exceeds uint32", opts.flags)
	}

	readOnly := opts.action == "list" || opts.action == "inspect"
	var conn *libvirt.Connect
	if readOnly {
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
		return listDomains(conn)
	case "define":
		return defineDomain(conn, opts.xml)
	case "inspect", "start", "shutdown", "destroy", "undefine":
		if opts.name == "" {
			return errors.New("-name is required for this action")
		}
		return actOnDomain(conn, opts)
	default:
		return fmt.Errorf("unknown action %q", opts.action)
	}
}

func listDomains(conn *libvirt.Connect) (err error) {
	domains, err := conn.ListAllDomains(0)
	if err != nil {
		return fmt.Errorf("list domains: %w", err)
	}
	defer func() {
		for _, domain := range domains {
			err = errors.Join(err, domain.Free())
		}
	}()

	for _, domain := range domains {
		if err := printDomain(domain, false); err != nil {
			return err
		}
	}
	return nil
}

func defineDomain(conn *libvirt.Connect, path string) (err error) {
	if path == "" {
		return errors.New("-xml is required for define")
	}
	document, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read domain XML: %w", err)
	}
	domain, err := conn.DefineDomainXML(string(document))
	if err != nil {
		return fmt.Errorf("define domain: %w", err)
	}
	defer func() { err = errors.Join(err, domain.Free()) }()

	return printDomain(domain, false)
}

func actOnDomain(conn *libvirt.Connect, opts options) (err error) {
	domain, err := conn.LookupDomainByName(opts.name)
	if err != nil {
		return fmt.Errorf("lookup domain %q: %w", opts.name, err)
	}
	defer func() { err = errors.Join(err, domain.Free()) }()

	switch opts.action {
	case "inspect":
		return printDomain(domain, true)
	case "start":
		err = domain.Create()
	case "shutdown":
		err = domain.Shutdown()
	case "destroy":
		err = domain.Destroy()
	case "undefine":
		if opts.flags == 0 {
			err = domain.Undefine()
		} else {
			err = domain.UndefineFlags(uint32(opts.flags))
		}
	}
	if err != nil {
		return fmt.Errorf("%s domain %q: %w", opts.action, opts.name, err)
	}
	fmt.Printf("%s requested for domain %s\n", opts.action, opts.name)
	return nil
}

func printDomain(domain *libvirt.Domain, includeXML bool) error {
	name, err := domain.GetName()
	if err != nil {
		return fmt.Errorf("get domain name: %w", err)
	}
	uuid, err := domain.GetUUIDString()
	if err != nil {
		return fmt.Errorf("get domain %q UUID: %w", name, err)
	}
	state, reason, err := domain.GetState()
	if err != nil {
		return fmt.Errorf("get domain %q state: %w", name, err)
	}
	active, err := domain.IsActive()
	if err != nil {
		return fmt.Errorf("check domain %q activity: %w", name, err)
	}
	fmt.Printf("%s uuid=%s state=%s reason=%d active=%t\n", name, uuid, stateName(state), reason, active)

	if includeXML {
		document, err := domain.GetXMLDesc(0)
		if err != nil {
			return fmt.Errorf("get domain %q XML: %w", name, err)
		}
		fmt.Println(document)
	}
	return nil
}

func stateName(state libvirt.DomainState) string {
	switch state {
	case libvirt.DomainNoState:
		return "no-state"
	case libvirt.DomainRunning:
		return "running"
	case libvirt.DomainBlocked:
		return "blocked"
	case libvirt.DomainPaused:
		return "paused"
	case libvirt.DomainShutdown:
		return "shutdown"
	case libvirt.DomainShutoff:
		return "shutoff"
	case libvirt.DomainCrashed:
		return "crashed"
	case libvirt.DomainPMSuspended:
		return "pm-suspended"
	default:
		return fmt.Sprintf("unknown(%d)", state)
	}
}
