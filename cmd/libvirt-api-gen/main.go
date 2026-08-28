package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Ns2Kracy/libvirt-go/internal/generator"
)

func main() {
	config := generator.Config{}
	flag.StringVar(&config.APIPath, "api", "auto", "path to libvirt-api.xml, or auto")
	flag.StringVar(&config.FunctionMode, "functions", "all", "function set to generate: all or used")
	flag.StringVar(&config.PackageDir, "package", ".", "directory containing the libvirt Go package")
	flag.StringVar(&config.Output, "out", generator.DefaultOutput, "generated Go output path")
	flag.Parse()

	if err := generator.Run(config); err != nil {
		fmt.Fprintln(os.Stderr, "libvirt-api-gen:", err)
		os.Exit(1)
	}
}
