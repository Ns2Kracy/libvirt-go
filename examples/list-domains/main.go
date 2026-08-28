package main

import (
	"fmt"
	"log"

	libvirt "github.com/Ns2Kracy/libvirt-go"
)

func main() {
	conn, err := libvirt.NewConnectReadOnly("test:///default")
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if _, err := conn.Close(); err != nil {
			log.Printf("close connection: %v", err)
		}
	}()

	domains, err := conn.ListAllDomains(0)
	if err != nil {
		log.Fatal(err)
	}
	for _, domain := range domains {
		name, nameErr := domain.GetName()
		if err := domain.Free(); err != nil {
			log.Printf("free domain: %v", err)
		}
		if nameErr != nil {
			log.Fatal(nameErr)
		}
		fmt.Println(name)
	}
}
