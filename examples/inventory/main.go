package main

import (
	"errors"
	"flag"
	"fmt"
	"log"

	libvirt "github.com/Ns2Kracy/libvirt-go"
)

func main() {
	uri := flag.String("uri", "test:///default", "libvirt connection URI")
	flag.Parse()

	if err := run(*uri); err != nil {
		log.Fatal(err)
	}
}

func run(uri string) (err error) {
	libraryVersion, err := libvirt.GetVersion()
	if err != nil {
		return fmt.Errorf("get local libvirt version: %w", err)
	}

	conn, err := libvirt.NewConnectReadOnly(uri)
	if err != nil {
		return fmt.Errorf("open %q: %w", uri, err)
	}
	defer func() {
		_, closeErr := conn.Close()
		err = errors.Join(err, closeErr)
	}()

	canonicalURI, err := conn.GetURI()
	if err != nil {
		return fmt.Errorf("get connection URI: %w", err)
	}
	hypervisorVersion, err := conn.GetVersion()
	if err != nil {
		return fmt.Errorf("get hypervisor version: %w", err)
	}
	connectionLibraryVersion, err := conn.GetLibVersion()
	if err != nil {
		return fmt.Errorf("get connection libvirt version: %w", err)
	}
	alive, err := conn.IsAlive()
	if err != nil {
		return fmt.Errorf("check connection: %w", err)
	}

	fmt.Printf("URI: %s\n", canonicalURI)
	fmt.Printf("Local libvirt: %s\n", libvirt.DecodeVersion(libraryVersion))
	fmt.Printf("Connection libvirt: %s\n", libvirt.DecodeVersion(connectionLibraryVersion))
	fmt.Printf("Hypervisor: %s\n", libvirt.DecodeVersion(hypervisorVersion))
	fmt.Printf("Alive: %t\n", alive)

	if err := printDomains(conn); err != nil {
		return err
	}
	if err := printNetworks(conn); err != nil {
		return err
	}
	if err := printStoragePools(conn); err != nil {
		return err
	}
	return nil
}

func printDomains(conn *libvirt.Connect) (err error) {
	domains, err := conn.ListAllDomains(0)
	if err != nil {
		return fmt.Errorf("list domains: %w", err)
	}
	defer func() {
		for _, domain := range domains {
			err = errors.Join(err, domain.Free())
		}
	}()

	fmt.Printf("\nDomains (%d):\n", len(domains))
	for _, domain := range domains {
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
			return fmt.Errorf("check domain %q: %w", name, err)
		}
		fmt.Printf("- %s uuid=%s state=%d reason=%d active=%t\n", name, uuid, state, reason, active)
	}
	return nil
}

func printNetworks(conn *libvirt.Connect) (err error) {
	networks, err := conn.ListAllNetworks(0)
	if err != nil {
		return fmt.Errorf("list networks: %w", err)
	}
	defer func() {
		for _, network := range networks {
			err = errors.Join(err, network.Free())
		}
	}()

	fmt.Printf("\nNetworks (%d):\n", len(networks))
	for _, network := range networks {
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
		fmt.Printf("- %s uuid=%s active=%t persistent=%t\n", name, uuid, active, persistent)
	}
	return nil
}

func printStoragePools(conn *libvirt.Connect) (err error) {
	pools, err := conn.ListAllStoragePools(0)
	if err != nil {
		return fmt.Errorf("list storage pools: %w", err)
	}
	defer func() {
		for _, pool := range pools {
			err = errors.Join(err, pool.Free())
		}
	}()

	fmt.Printf("\nStorage pools (%d):\n", len(pools))
	for _, pool := range pools {
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
		fmt.Printf("- %s uuid=%s active=%t persistent=%t\n", name, uuid, active, persistent)
	}
	return nil
}
