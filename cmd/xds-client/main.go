package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	driver "github.com/caitong93/xds-client/driver"
	adsc "github.com/caitong93/xds-client/xds"

	v2 "github.com/envoyproxy/go-control-plane/envoy/api/v2"
)

type BootstrapConfig struct {
	HttpAddr            string
	ControlPlaneAddress string
	ClientsNum          int
}

var (
	xdsClient *adsc.ADSC
	xdsLoad   []*adsc.ADSC

	bootstrap           BootstrapConfig
	controlPlaneAddress string
)

func init() {
	flag.StringVar(&bootstrap.HttpAddr, "http-addr", "127.0.0.1:8080", "http address")
	flag.StringVar(&bootstrap.ControlPlaneAddress, "pilot-address", "", "control plane address")
	flag.IntVar(&bootstrap.ClientsNum, "clients", 1, "concurrent clients number")
}

func sendRequests(node *driver.Node, d driver.Driver) error {
	// Send CDS & EDS
	if err := d.SendRequest(node, &v2.DiscoveryRequest{
		TypeUrl: "type.googleapis.com/envoy.api.v2.Cluster",
	}); err != nil {
		return err
	}

	// Send LDS & RDS
	if err := d.SendRequest(node, &v2.DiscoveryRequest{
		TypeUrl: "type.googleapis.com/envoy.api.v2.Listener",
	}); err != nil {
		return err
	}

	return nil
}

func run(stopCh chan struct{}) error {
	d := driver.New(bootstrap.ControlPlaneAddress)
	defer d.Close()

	for i := 0; i < bootstrap.ClientsNum; i++ {
		n := driver.RandomNode()
		log.Println("Add client", i)
		if err := d.AddClient(n); err != nil {
			return err
		}

		if err := sendRequests(n, d); err != nil {
			return err
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/configdump", http.HandlerFunc(d.GetConfigDumpHandler))

	go func() {
		err := http.ListenAndServe(bootstrap.HttpAddr, mux)
		log.Fatal(err)
	}()

	<-stopCh

	return nil
}

func main() {
	flag.Parse()

	stopCh := SetupSignalHandler()

	if err := run(stopCh); err != nil {
		fmt.Println(err)
	}
}
