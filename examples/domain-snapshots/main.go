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
	uri        string
	domainName string
	kind       string
	action     string
	name       string
	xml        string
	flags      uint
}

func main() {
	var opts options
	flag.StringVar(&opts.uri, "uri", "test:///default", "libvirt connection URI")
	flag.StringVar(&opts.domainName, "domain", "", "domain name")
	flag.StringVar(&opts.kind, "kind", "snapshot", "snapshot or checkpoint")
	flag.StringVar(&opts.action, "action", "list", "list, create, inspect, current, children, revert, or delete")
	flag.StringVar(&opts.name, "name", "", "snapshot or checkpoint name")
	flag.StringVar(&opts.xml, "xml", "", "XML file used by create")
	flag.UintVar(&opts.flags, "flags", 0, "libvirt operation flags as an unsigned integer")
	flag.Parse()

	if err := run(opts); err != nil {
		log.Fatal(err)
	}
}

func run(opts options) (err error) {
	opts.kind = strings.ToLower(opts.kind)
	opts.action = strings.ToLower(opts.action)
	if opts.domainName == "" {
		return errors.New("-domain is required")
	}
	if opts.flags > math.MaxUint32 {
		return fmt.Errorf("flags value %d exceeds uint32", opts.flags)
	}

	readOnly := opts.action == "list" || opts.action == "inspect" || opts.action == "current" || opts.action == "children"
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

	domain, err := conn.LookupDomainByName(opts.domainName)
	if err != nil {
		return fmt.Errorf("lookup domain %q: %w", opts.domainName, err)
	}
	defer func() { err = errors.Join(err, domain.Free()) }()

	switch opts.kind {
	case "snapshot":
		return runSnapshotAction(domain, opts)
	case "checkpoint":
		return runCheckpointAction(domain, opts)
	default:
		return fmt.Errorf("unknown kind %q", opts.kind)
	}
}

func runSnapshotAction(domain *libvirt.Domain, opts options) error {
	flags := uint32(opts.flags)
	switch opts.action {
	case "list":
		return listSnapshots(domain, flags)
	case "create":
		document, err := readXML(opts.xml)
		if err != nil {
			return err
		}
		snapshot, err := domain.CreateSnapshotXML(document, flags)
		if err != nil {
			return fmt.Errorf("create snapshot: %w", err)
		}
		return useSnapshot(snapshot, printSnapshot)
	case "current":
		snapshot, err := domain.CurrentSnapshot(flags)
		if err != nil {
			return fmt.Errorf("get current snapshot: %w", err)
		}
		return useSnapshot(snapshot, printSnapshot)
	case "inspect", "revert", "delete":
		if opts.name == "" {
			return errors.New("-name is required for this action")
		}
		snapshot, err := domain.LookupSnapshotByName(opts.name, flags)
		if err != nil {
			return fmt.Errorf("lookup snapshot %q: %w", opts.name, err)
		}
		return useSnapshot(snapshot, func(snapshot *libvirt.DomainSnapshot) error {
			switch opts.action {
			case "inspect":
				return printSnapshot(snapshot)
			case "revert":
				if err := snapshot.Revert(flags); err != nil {
					return fmt.Errorf("revert snapshot %q: %w", opts.name, err)
				}
			case "delete":
				if err := snapshot.Delete(flags); err != nil {
					return fmt.Errorf("delete snapshot %q: %w", opts.name, err)
				}
			}
			fmt.Printf("%s snapshot %s\n", opts.action, opts.name)
			return nil
		})
	default:
		return fmt.Errorf("action %q is not valid for snapshots", opts.action)
	}
}

func runCheckpointAction(domain *libvirt.Domain, opts options) error {
	flags := uint32(opts.flags)
	switch opts.action {
	case "list":
		return listCheckpoints(domain, flags)
	case "create":
		document, err := readXML(opts.xml)
		if err != nil {
			return err
		}
		checkpoint, err := domain.CreateCheckpointXML(document, flags)
		if err != nil {
			return fmt.Errorf("create checkpoint: %w", err)
		}
		return useCheckpoint(checkpoint, printCheckpoint)
	case "inspect", "children", "delete":
		if opts.name == "" {
			return errors.New("-name is required for this action")
		}
		checkpoint, err := domain.LookupCheckpointByName(opts.name, flags)
		if err != nil {
			return fmt.Errorf("lookup checkpoint %q: %w", opts.name, err)
		}
		return useCheckpoint(checkpoint, func(checkpoint *libvirt.DomainCheckpoint) error {
			switch opts.action {
			case "inspect":
				return printCheckpoint(checkpoint)
			case "children":
				return listCheckpointChildren(checkpoint, flags)
			case "delete":
				if err := checkpoint.Delete(flags); err != nil {
					return fmt.Errorf("delete checkpoint %q: %w", opts.name, err)
				}
				fmt.Printf("delete checkpoint %s\n", opts.name)
				return nil
			}
			return nil
		})
	default:
		return fmt.Errorf("action %q is not valid for checkpoints", opts.action)
	}
}

func listSnapshots(domain *libvirt.Domain, flags uint32) (err error) {
	snapshots, err := domain.ListAllSnapshots(flags)
	if err != nil {
		return fmt.Errorf("list snapshots: %w", err)
	}
	defer func() {
		for _, snapshot := range snapshots {
			err = errors.Join(err, snapshot.Free())
		}
	}()
	for _, snapshot := range snapshots {
		name, err := snapshot.GetName()
		if err != nil {
			return fmt.Errorf("get snapshot name: %w", err)
		}
		fmt.Println(name)
	}
	return nil
}

func listCheckpoints(domain *libvirt.Domain, flags uint32) (err error) {
	checkpoints, err := domain.ListAllCheckpoints(flags)
	if err != nil {
		return fmt.Errorf("list checkpoints: %w", err)
	}
	defer func() {
		for _, checkpoint := range checkpoints {
			err = errors.Join(err, checkpoint.Free())
		}
	}()
	for _, checkpoint := range checkpoints {
		name, err := checkpoint.GetName()
		if err != nil {
			return fmt.Errorf("get checkpoint name: %w", err)
		}
		fmt.Println(name)
	}
	return nil
}

func listCheckpointChildren(checkpoint *libvirt.DomainCheckpoint, flags uint32) (err error) {
	children, err := checkpoint.ListAllChildren(flags)
	if err != nil {
		return fmt.Errorf("list checkpoint children: %w", err)
	}
	defer func() {
		for _, child := range children {
			err = errors.Join(err, child.Free())
		}
	}()
	for _, child := range children {
		name, err := child.GetName()
		if err != nil {
			return fmt.Errorf("get child checkpoint name: %w", err)
		}
		fmt.Println(name)
	}
	return nil
}

func useSnapshot(snapshot *libvirt.DomainSnapshot, use func(*libvirt.DomainSnapshot) error) (err error) {
	defer func() { err = errors.Join(err, snapshot.Free()) }()
	return use(snapshot)
}

func useCheckpoint(checkpoint *libvirt.DomainCheckpoint, use func(*libvirt.DomainCheckpoint) error) (err error) {
	defer func() { err = errors.Join(err, checkpoint.Free()) }()
	return use(checkpoint)
}

func printSnapshot(snapshot *libvirt.DomainSnapshot) error {
	name, err := snapshot.GetName()
	if err != nil {
		return fmt.Errorf("get snapshot name: %w", err)
	}
	document, err := snapshot.GetXMLDesc(0)
	if err != nil {
		return fmt.Errorf("get snapshot %q XML: %w", name, err)
	}
	fmt.Printf("snapshot %s\n%s\n", name, document)
	return nil
}

func printCheckpoint(checkpoint *libvirt.DomainCheckpoint) error {
	name, err := checkpoint.GetName()
	if err != nil {
		return fmt.Errorf("get checkpoint name: %w", err)
	}
	document, err := checkpoint.GetXMLDesc(0)
	if err != nil {
		return fmt.Errorf("get checkpoint %q XML: %w", name, err)
	}
	fmt.Printf("checkpoint %s\n%s\n", name, document)
	return nil
}

func readXML(path string) (string, error) {
	if path == "" {
		return "", errors.New("-xml is required for create")
	}
	document, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read XML: %w", err)
	}
	return string(document), nil
}
