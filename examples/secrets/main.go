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
	uuid      string
	xml       string
	valueFile string
	flags     uint
}

func main() {
	var opts options
	flag.StringVar(&opts.uri, "uri", "qemu:///session", "libvirt connection URI")
	flag.StringVar(&opts.action, "action", "list", "list, inspect, define, set, get, or undefine")
	flag.StringVar(&opts.uuid, "uuid", "", "secret UUID")
	flag.StringVar(&opts.xml, "xml", "", "secret XML file used by define")
	flag.StringVar(&opts.valueFile, "value-file", "", "file read by set or written with mode 0600 by get")
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
	if opts.action == "list" || opts.action == "inspect" || opts.action == "get" {
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
		return listSecrets(conn, uint32(opts.flags))
	case "define":
		return defineSecret(conn, opts)
	case "inspect", "set", "get", "undefine":
		if opts.uuid == "" {
			return errors.New("-uuid is required for this action")
		}
		return actOnSecret(conn, opts)
	default:
		return fmt.Errorf("unknown action %q", opts.action)
	}
}

func listSecrets(conn *libvirt.Connect, flags uint32) (err error) {
	secrets, err := conn.ListAllSecrets(flags)
	if err != nil {
		return fmt.Errorf("list secrets: %w", err)
	}
	defer func() {
		for _, secret := range secrets {
			err = errors.Join(err, secret.Free())
		}
	}()
	for _, secret := range secrets {
		uuid, err := secret.GetUUIDString()
		if err != nil {
			return fmt.Errorf("get secret UUID: %w", err)
		}
		fmt.Println(uuid)
	}
	return nil
}

func defineSecret(conn *libvirt.Connect, opts options) (err error) {
	document, err := readFile(opts.xml, "secret XML")
	if err != nil {
		return err
	}
	secret, err := conn.DefineSecretXML(string(document), uint32(opts.flags))
	if err != nil {
		return fmt.Errorf("define secret: %w", err)
	}
	defer func() { err = errors.Join(err, secret.Free()) }()

	uuid, err := secret.GetUUIDString()
	if err != nil {
		return fmt.Errorf("get defined secret UUID: %w", err)
	}
	fmt.Println(uuid)
	return nil
}

func actOnSecret(conn *libvirt.Connect, opts options) (err error) {
	secret, err := conn.LookupSecretByUUIDString(opts.uuid)
	if err != nil {
		return fmt.Errorf("lookup secret %q: %w", opts.uuid, err)
	}
	defer func() { err = errors.Join(err, secret.Free()) }()

	flags := uint32(opts.flags)
	switch opts.action {
	case "inspect":
		document, err := secret.GetXMLDesc(flags)
		if err != nil {
			return fmt.Errorf("get secret XML: %w", err)
		}
		fmt.Println(document)
	case "set":
		value, err := readFile(opts.valueFile, "secret value")
		if err != nil {
			return err
		}
		if err := secret.SetValue(value, flags); err != nil {
			return fmt.Errorf("set secret value: %w", err)
		}
		fmt.Printf("set %d secret bytes\n", len(value))
	case "get":
		if opts.valueFile == "" {
			return errors.New("-value-file is required for get")
		}
		value, err := secret.GetValue(flags)
		if err != nil {
			return fmt.Errorf("get secret value: %w", err)
		}
		if err := writeSecretFile(opts.valueFile, value); err != nil {
			return err
		}
		fmt.Printf("wrote %d secret bytes\n", len(value))
	case "undefine":
		if err := secret.Undefine(); err != nil {
			return fmt.Errorf("undefine secret: %w", err)
		}
		fmt.Printf("undefined secret %s\n", opts.uuid)
	}
	return nil
}

func readFile(path, description string) ([]byte, error) {
	if path == "" {
		return nil, fmt.Errorf("file path is required for %s", description)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", description, err)
	}
	return content, nil
}

func writeSecretFile(path string, value []byte) (err error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open secret value file: %w", err)
	}
	defer func() { err = errors.Join(err, file.Close()) }()

	if err := file.Chmod(0o600); err != nil {
		return fmt.Errorf("restrict secret value file: %w", err)
	}
	if _, err := file.Write(value); err != nil {
		return fmt.Errorf("write secret value: %w", err)
	}
	return nil
}
