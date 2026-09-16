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
	action     string
	pool       string
	volume     string
	volumeKey  string
	volumePath string
	xml        string
	autostart  bool
	capacity   uint64
	flags      uint
}

func main() {
	var opts options
	flag.StringVar(&opts.uri, "uri", "test:///default", "libvirt connection URI")
	flag.StringVar(&opts.action, "action", "list-pools", "list-pools, inspect-pool, define-pool, create-pool, start-pool, stop-pool, autostart-pool, refresh-pool, undefine-pool, list-volumes, create-volume, inspect-volume, resize-volume, wipe-volume, or delete-volume")
	flag.StringVar(&opts.pool, "pool", "", "storage pool name")
	flag.StringVar(&opts.volume, "volume", "", "volume name within -pool")
	flag.StringVar(&opts.volumeKey, "volume-key", "", "globally unique volume key (alternative to -pool and -volume)")
	flag.StringVar(&opts.volumePath, "volume-path", "", "volume path (alternative to -pool and -volume)")
	flag.StringVar(&opts.xml, "xml", "", "storage pool or volume XML file")
	flag.BoolVar(&opts.autostart, "autostart", true, "autostart value used by autostart-pool")
	flag.Uint64Var(&opts.capacity, "capacity", 0, "new volume capacity in bytes used by resize-volume")
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
	case "list-pools":
		return listPools(conn)
	case "define-pool", "create-pool":
		return createPool(conn, opts)
	case "inspect-pool", "start-pool", "stop-pool", "autostart-pool", "refresh-pool", "undefine-pool", "list-volumes", "create-volume":
		return actOnPool(conn, opts)
	case "inspect-volume", "resize-volume", "wipe-volume", "delete-volume":
		return actOnVolume(conn, opts)
	default:
		return fmt.Errorf("unknown action %q", opts.action)
	}
}

func isReadOnlyAction(action string) bool {
	switch action {
	case "list-pools", "inspect-pool", "list-volumes", "inspect-volume":
		return true
	default:
		return false
	}
}

func listPools(conn *libvirt.Connect) (err error) {
	pools, err := conn.ListAllStoragePools(0)
	if err != nil {
		return fmt.Errorf("list storage pools: %w", err)
	}
	defer func() {
		for _, pool := range pools {
			err = errors.Join(err, pool.Free())
		}
	}()
	for i := range pools {
		if err := printPool(&pools[i], false); err != nil {
			return err
		}
	}
	return nil
}

func createPool(conn *libvirt.Connect, opts options) (err error) {
	document, err := readXML(opts.xml)
	if err != nil {
		return err
	}

	var pool *libvirt.StoragePool
	if opts.action == "define-pool" {
		pool, err = conn.StoragePoolDefineXML(document, uint32(opts.flags))
	} else {
		pool, err = conn.StoragePoolCreateXML(document, uint32(opts.flags))
	}
	if err != nil {
		return fmt.Errorf("%s: %w", opts.action, err)
	}
	defer func() { err = errors.Join(err, pool.Free()) }()
	return printPool(pool, false)
}

func actOnPool(conn *libvirt.Connect, opts options) (err error) {
	if opts.pool == "" {
		return errors.New("-pool is required for this action")
	}
	pool, err := conn.LookupStoragePoolByName(opts.pool)
	if err != nil {
		return fmt.Errorf("lookup storage pool %q: %w", opts.pool, err)
	}
	defer func() { err = errors.Join(err, pool.Free()) }()

	flags := uint32(opts.flags)
	switch opts.action {
	case "inspect-pool":
		return printPool(pool, true)
	case "start-pool":
		err = pool.Create(flags)
	case "stop-pool":
		err = pool.Destroy()
	case "autostart-pool":
		err = pool.SetAutostart(opts.autostart)
	case "refresh-pool":
		err = pool.Refresh(flags)
	case "undefine-pool":
		err = pool.Undefine()
	case "list-volumes":
		return listVolumes(pool, flags)
	case "create-volume":
		document, readErr := readXML(opts.xml)
		if readErr != nil {
			return readErr
		}
		volume, createErr := pool.StorageVolCreateXML(document, flags)
		if createErr != nil {
			return fmt.Errorf("create volume: %w", createErr)
		}
		return useVolume(volume, printVolume)
	}
	if err != nil {
		return fmt.Errorf("%s %q: %w", opts.action, opts.pool, err)
	}
	fmt.Printf("%s requested for storage pool %s\n", opts.action, opts.pool)
	return nil
}

func actOnVolume(conn *libvirt.Connect, opts options) (err error) {
	volume, pool, err := lookupVolume(conn, opts)
	if err != nil {
		return err
	}
	if pool != nil {
		defer func() { err = errors.Join(err, pool.Free()) }()
	}
	defer func() { err = errors.Join(err, volume.Free()) }()

	flags := uint32(opts.flags)
	switch opts.action {
	case "inspect-volume":
		return printVolume(volume)
	case "resize-volume":
		if opts.capacity == 0 {
			return errors.New("-capacity must be greater than zero")
		}
		err = volume.Resize(opts.capacity, flags)
	case "wipe-volume":
		err = volume.Wipe(flags)
	case "delete-volume":
		err = volume.Delete(flags)
	}
	if err != nil {
		return fmt.Errorf("%s: %w", opts.action, err)
	}
	fmt.Printf("%s requested\n", opts.action)
	return nil
}

func lookupVolume(conn *libvirt.Connect, opts options) (*libvirt.StorageVol, *libvirt.StoragePool, error) {
	if opts.volumeKey != "" {
		volume, err := conn.LookupStorageVolByKey(opts.volumeKey)
		return volume, nil, err
	}
	if opts.volumePath != "" {
		volume, err := conn.LookupStorageVolByPath(opts.volumePath)
		return volume, nil, err
	}
	if opts.pool == "" || opts.volume == "" {
		return nil, nil, errors.New("use -volume-key, -volume-path, or both -pool and -volume")
	}
	pool, err := conn.LookupStoragePoolByName(opts.pool)
	if err != nil {
		return nil, nil, fmt.Errorf("lookup storage pool %q: %w", opts.pool, err)
	}
	volume, err := pool.LookupStorageVolByName(opts.volume)
	if err != nil {
		_ = pool.Free()
		return nil, nil, fmt.Errorf("lookup volume %q: %w", opts.volume, err)
	}
	return volume, pool, nil
}

func listVolumes(pool *libvirt.StoragePool, flags uint32) (err error) {
	volumes, err := pool.ListAllStorageVolumes(flags)
	if err != nil {
		return fmt.Errorf("list storage volumes: %w", err)
	}
	defer func() {
		for _, volume := range volumes {
			err = errors.Join(err, volume.Free())
		}
	}()
	for i := range volumes {
		if err := printVolume(&volumes[i]); err != nil {
			return err
		}
	}
	return nil
}

func useVolume(volume *libvirt.StorageVol, use func(*libvirt.StorageVol) error) (err error) {
	defer func() { err = errors.Join(err, volume.Free()) }()
	return use(volume)
}

func printPool(pool *libvirt.StoragePool, includeXML bool) error {
	name, err := pool.GetName()
	if err != nil {
		return fmt.Errorf("get storage pool name: %w", err)
	}
	uuid, err := pool.GetUUIDString()
	if err != nil {
		return fmt.Errorf("get storage pool %q UUID: %w", name, err)
	}
	active, err := pool.IsActive()
	if err != nil {
		return fmt.Errorf("check storage pool %q activity: %w", name, err)
	}
	persistent, err := pool.IsPersistent()
	if err != nil {
		return fmt.Errorf("check storage pool %q persistence: %w", name, err)
	}
	fmt.Printf("%s uuid=%s active=%t persistent=%t", name, uuid, active, persistent)
	if persistent {
		autostart, err := pool.GetAutostart()
		if err != nil {
			return fmt.Errorf("get storage pool %q autostart: %w", name, err)
		}
		fmt.Printf(" autostart=%t", autostart)
	}
	fmt.Println()

	if includeXML {
		document, err := pool.GetXMLDesc(0)
		if err != nil {
			return fmt.Errorf("get storage pool %q XML: %w", name, err)
		}
		fmt.Println(document)
	}
	return nil
}

func printVolume(volume *libvirt.StorageVol) error {
	name, err := volume.GetName()
	if err != nil {
		return fmt.Errorf("get volume name: %w", err)
	}
	key, err := volume.GetKey()
	if err != nil {
		return fmt.Errorf("get volume %q key: %w", name, err)
	}
	path, err := volume.GetPath()
	if err != nil {
		return fmt.Errorf("get volume %q path: %w", name, err)
	}
	document, err := volume.GetXMLDesc(0)
	if err != nil {
		return fmt.Errorf("get volume %q XML: %w", name, err)
	}
	fmt.Printf("%s key=%s path=%s\n%s\n", name, key, path, document)
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
