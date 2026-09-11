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
	uri       string
	action    string
	name      string
	xml       string
	portUUID  string
	autostart bool
	flags     uint
}

func main() {
	var opts options
	flag.StringVar(&opts.uri, "uri", "test:///default", "libvirt connection URI")
	flag.StringVar(&opts.action, "action", "list", "list, inspect, define, create, start, stop, autostart, undefine, list-ports, create-port, inspect-port, or delete-port")
	flag.StringVar(&opts.name, "name", "", "network name")
	flag.StringVar(&opts.xml, "xml", "", "network or port XML file used by define/create actions")
	flag.StringVar(&opts.portUUID, "port", "", "network port UUID")
	flag.BoolVar(&opts.autostart, "autostart", true, "autostart value used by the autostart action")
	flag.UintVar(&opts.flags, "flags", 0, "libvirt operation flags as an unsigned integer")
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

	var conn *libvirt.Connect
	if isReadOnlyAction(opts.action) {
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
		return listNetworks(conn)
	case "define", "create":
		return createNetwork(conn, opts)
	case "inspect", "start", "stop", "autostart", "undefine", "list-ports", "create-port", "inspect-port", "delete-port":
		if opts.name == "" {
			return errors.New("-name is required for this action")
		}
		return actOnNetwork(conn, opts)
	default:
		return fmt.Errorf("unknown action %q", opts.action)
	}
}

func isReadOnlyAction(action string) bool {
	switch action {
	case "list", "inspect", "list-ports", "inspect-port":
		return true
	default:
		return false
	}
}

func listNetworks(conn *libvirt.Connect) (err error) {
	networks, err := conn.ListAllNetworks(0)
	if err != nil {
		return fmt.Errorf("list networks: %w", err)
	}
	defer func() {
		for _, network := range networks {
			err = errors.Join(err, network.Free())
		}
	}()

	for _, network := range networks {
		if err := printNetwork(network, false); err != nil {
			return err
		}
	}
	return nil
}

func createNetwork(conn *libvirt.Connect, opts options) (err error) {
	document, err := readXML(opts.xml)
	if err != nil {
		return err
	}

	var network *libvirt.Network
	if opts.action == "define" {
		network, err = conn.DefineNetworkXML(document)
	} else {
		network, err = conn.CreateNetworkXML(document)
	}
	if err != nil {
		return fmt.Errorf("%s network: %w", opts.action, err)
	}
	defer func() { err = errors.Join(err, network.Free()) }()
	return printNetwork(network, false)
}

func actOnNetwork(conn *libvirt.Connect, opts options) (err error) {
	network, err := conn.LookupNetworkByName(opts.name)
	if err != nil {
		return fmt.Errorf("lookup network %q: %w", opts.name, err)
	}
	defer func() { err = errors.Join(err, network.Free()) }()

	flags := uint32(opts.flags)
	switch opts.action {
	case "inspect":
		return printNetwork(network, true)
	case "start":
		err = network.Create()
	case "stop":
		err = network.Destroy()
	case "autostart":
		err = network.SetAutostart(opts.autostart)
	case "undefine":
		err = network.Undefine()
	case "list-ports":
		return listPorts(network, flags)
	case "create-port":
		document, readErr := readXML(opts.xml)
		if readErr != nil {
			return readErr
		}
		port, createErr := network.CreatePortXML(document, flags)
		if createErr != nil {
			return fmt.Errorf("create port: %w", createErr)
		}
		return usePort(port, printPort)
	case "inspect-port", "delete-port":
		if opts.portUUID == "" {
			return errors.New("-port is required for this action")
		}
		port, lookupErr := network.LookupPortByUUIDString(opts.portUUID)
		if lookupErr != nil {
			return fmt.Errorf("lookup port %q: %w", opts.portUUID, lookupErr)
		}
		return usePort(port, func(port *libvirt.NetworkPort) error {
			if opts.action == "inspect-port" {
				return printPort(port)
			}
			if err := port.Delete(flags); err != nil {
				return fmt.Errorf("delete port %q: %w", opts.portUUID, err)
			}
			fmt.Printf("deleted port %s\n", opts.portUUID)
			return nil
		})
	}
	if err != nil {
		return fmt.Errorf("%s network %q: %w", opts.action, opts.name, err)
	}
	fmt.Printf("%s requested for network %s\n", opts.action, opts.name)
	return nil
}

func listPorts(network *libvirt.Network, flags uint32) (err error) {
	ports, err := network.ListAllPorts(flags)
	if err != nil {
		return fmt.Errorf("list network ports: %w", err)
	}
	defer func() {
		for _, port := range ports {
			err = errors.Join(err, port.Free())
		}
	}()
	for _, port := range ports {
		uuid, err := port.GetUUIDString()
		if err != nil {
			return fmt.Errorf("get port UUID: %w", err)
		}
		fmt.Println(uuid)
	}
	return nil
}

func usePort(port *libvirt.NetworkPort, use func(*libvirt.NetworkPort) error) (err error) {
	defer func() { err = errors.Join(err, port.Free()) }()
	return use(port)
}

func printNetwork(network *libvirt.Network, includeXML bool) error {
	name, err := network.GetName()
	if err != nil {
		return fmt.Errorf("get network name: %w", err)
	}
	uuid, err := network.GetUUIDString()
	if err != nil {
		return fmt.Errorf("get network %q UUID: %w", name, err)
	}
	active, err := network.IsActive()
	if err != nil {
		return fmt.Errorf("check network %q activity: %w", name, err)
	}
	persistent, err := network.IsPersistent()
	if err != nil {
		return fmt.Errorf("check network %q persistence: %w", name, err)
	}
	fmt.Printf("%s uuid=%s active=%t persistent=%t", name, uuid, active, persistent)
	if persistent {
		autostart, err := network.GetAutostart()
		if err != nil {
			return fmt.Errorf("get network %q autostart: %w", name, err)
		}
		fmt.Printf(" autostart=%t", autostart)
	}
	fmt.Println()

	if includeXML {
		document, err := network.GetXMLDesc(0)
		if err != nil {
			return fmt.Errorf("get network %q XML: %w", name, err)
		}
		fmt.Println(document)
	}
	return nil
}

func printPort(port *libvirt.NetworkPort) error {
	uuid, err := port.GetUUIDString()
	if err != nil {
		return fmt.Errorf("get port UUID: %w", err)
	}
	document, err := port.GetXMLDesc(0)
	if err != nil {
		return fmt.Errorf("get port %q XML: %w", uuid, err)
	}
	fmt.Printf("port %s\n%s\n", uuid, document)
	return nil
}

func readXML(path string) (string, error) {
	if path == "" {
		return "", errors.New("-xml is required for this action")
	}
	document, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read XML: %w", err)
	}
	return string(document), nil
}
