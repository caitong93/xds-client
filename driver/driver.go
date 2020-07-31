package driver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	adsc "github.com/caitong93/xds-client/xds"
	v2 "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/google/uuid"
	"istio.io/istio/pilot/pkg/model"
)

type Node struct {
	Metadata  model.NodeMetadata `json:"metadata"`
	Workload  string             `json:"workload"`
	Namespace string             `json:"namespace"`
	NodeType  string             `json:"nodetype"`
	IP        string             `json:"ip"`

	uuid      string
	adsClient *adsc.ADSC
}

type NodeMeta struct {
	Labels  map[string]string `json:"labels"`
	Version string            `json:"version"`
}

func RandomNode() *Node {
	return randomNode()
}

func randomNode() *Node {
	var node Node

	node.NodeType = "sidecar"
	node.Namespace = "default"
	workload := "fake-workload-" + uuid.New().String()
	node.Workload = workload
	node.Metadata.Labels = make(map[string]string)
	node.Metadata.Labels["version"] = "v1"
	node.Metadata.Labels["LABELS"] = fmt.Sprintf(`{"app":"%s"}`, node.Workload)

	return &node
}

type Driver interface {
	AddClient(node *Node) error
	SendRequest(node *Node, req *v2.DiscoveryRequest) error
	GetConfigDumpHandler(w http.ResponseWriter, req *http.Request)
	Close()
}

// driver manages multiple xds client.
type driver struct {
	controlPlaneAddress string
	clients             map[string]*Node
}

// New creates a new driver.
func New(controlPlaneAddress string) Driver {
	return newDriver(controlPlaneAddress)
}

func newDriver(address string) *driver {
	return &driver{
		controlPlaneAddress: address,
		clients:             make(map[string]*Node),
	}
}

func (d *driver) AddClient(node *Node) error {
	return d.initClient(node)
}

func (d *driver) initClient(node *Node) error {
	if node.uuid != "" {
		node.uuid = uuid.New().String()
	}

	adsConfig := &adsc.Config{
		Workload:  node.Workload,
		Namespace: node.Namespace,
		IP:        node.IP,
		NodeType:  node.NodeType,
		Meta:      node.Metadata.ToStruct(),
	}

	adsClient, err := connectAds(d.controlPlaneAddress, adsConfig)
	if err != nil {
		return err
	}

	node.adsClient = adsClient
	d.clients[node.uuid] = node

	go d.handleClose(node, adsClient.Updates)

	return nil
}

func (d *driver) SendRequest(node *Node, req *v2.DiscoveryRequest) error {
	n, exist := d.clients[node.uuid]
	if !exist {
		return fmt.Errorf("node not found")
	}

	if err := n.adsClient.Send(req); err != nil {
		return err
	}
	return nil
}

func (d *driver) handleClose(node *Node, updates chan string) {
	for msg := range updates {
		if msg == "close" {
			delete(d.clients, node.uuid)
			node.adsClient = nil
			go d.retry(func() error {
				return d.initClient(node)
			})
		}
	}
}

func (d *driver) retry(f func() error) {
	for {
		err := f()
		if err == nil {
			return
		}

		time.Sleep(20 * time.Millisecond)
	}
}

func connectAds(address string, config *adsc.Config) (*adsc.ADSC, error) {
	adsClient, err := adsc.Dial(address, "", config)
	if err != nil {
		return nil, err
	}

	return adsClient, nil
}

func (d *driver) GetConfigDumpHandler(w http.ResponseWriter, req *http.Request) {
	d.getConfigDumpHandler(w, req)
}

func (d *driver) getConfigDumpHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	var result []*ConfigDump

	for _, node := range d.clients {
		result = append(result, d.getNodeConfigDump(node))
	}

	data, err := json.Marshal(result)
	if err != nil {
		fmt.Println("error marshal:", err)
		w.WriteHeader(500)
		return
	}

	w.Write(data)
}

func (d *driver) Close() {
	for _, n := range d.clients {
		n.adsClient.Close()
	}
}

type ConfigDump struct {
	NodeInfo      *Node       `json:"node"`
	HttpListeners interface{} `json:"http_listeners"`
	TcpListeners  interface{} `json:"tcp_listeners"`
	Clusters      interface{} `json:"clusters"`
	EdsClusters   interface{} `json:"edsClusters"`
	Routes        interface{} `json:"routes"`
	Endpoints     interface{} `json:"endpoints"`
}

func (d *driver) getNodeConfigDump(node *Node) *ConfigDump {
	var configDump ConfigDump
	configDump.NodeInfo = node

	configDump.HttpListeners = node.adsClient.GetHTTPListeners()
	configDump.TcpListeners = node.adsClient.GetTCPListeners()
	configDump.Clusters = node.adsClient.GetClusters()
	configDump.EdsClusters = node.adsClient.GetEdsClusters()
	configDump.Routes = node.adsClient.GetRoutes()
	configDump.Endpoints = node.adsClient.GetEndpoints()

	return &configDump
}
