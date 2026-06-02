package main

import (
	"flag"
	"fmt"
	"os"
	"time"
)

const (
	version = "0.1.0"

	schemaVersion = 1

	defaultBackendAddr = "127.0.0.1:6668"
)

func main() {
	addr := flag.String("addr", defaultBackendAddr, "Atmos backend pubsub address")
	timeout := flag.Duration("timeout", 2*time.Second, "TCP connect/write timeout")
	jsonOutput := flag.Bool("json", false, "print machine-readable JSON")
	flag.Parse()

	if flag.NArg() < 1 {
		usage()
		os.Exit(2)
	}

	var err error
	switch flag.Arg(0) {
	case "version":
		if flag.NArg() != 1 {
			usage()
			os.Exit(2)
		}
		if *jsonOutput {
			err = printJSON(versionOutput{
				SchemaVersion: schemaVersion,
				Version:       version,
			})
			break
		}
		fmt.Printf("atmosctl %s\n", version)
		return
	case "vpn":
		if flag.NArg() != 2 {
			usage()
			os.Exit(2)
		}
		err = handleVPN(flag.Arg(1), *addr, *timeout, *jsonOutput)
	case "autostart":
		if flag.NArg() != 2 {
			usage()
			os.Exit(2)
		}
		err = handleAutostart(flag.Arg(1), *jsonOutput)
	default:
		usage()
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: %s [--addr %s] [--timeout 2s] [--json] version | vpn status|pause|resume | autostart status|enable|disable\n", os.Args[0], defaultBackendAddr)
}
