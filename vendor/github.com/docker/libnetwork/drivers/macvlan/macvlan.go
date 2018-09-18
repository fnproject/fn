package macvlan

import (
	"net"
	"sync"

	"github.com/docker/libnetwork/datastore"
	"github.com/docker/libnetwork/discoverapi"
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/osl"
	"github.com/docker/libnetwork/types"
)

const (
	vethLen             = 7
	containerVethPrefix = "eth"
	vethPrefix          = "veth"
	macvlanType         = "macvlan"  // driver type name
	modePrivate         = "private"  // macvlan mode private
	modeVepa            = "vepa"     // macvlan mode vepa
	modeBridge          = "bridge"   // macvlan mode bridge
	modePassthru        = "passthru" // macvlan mode passthrough
	parentOpt           = "parent"   // parent interface -o parent
	modeOpt             = "_mode"    // macvlan mode ux opt suffix
)

var driverModeOpt = macvlanType + modeOpt // mode --option macvlan_mode

type endpointTable map[string]*endpoint

type networkTable map[string]*network

type driver struct {
	networks networkTable
	sync.Once
	sync.Mutex
	store datastore.DataStore
}

type endpoint struct {
	id      string
	mac     net.HardwareAddr
	addr    *net.IPNet
	addrv6  *net.IPNet
	srcName string
}

type network struct {
	id        string
	sbox      osl.Sandbox
	endpoints endpointTable
	driver    *driver
	config    *configuration
	sync.Mutex
}

// Init initializes and registers the libnetwork macvlan driver
func Init(dc driverapi.DriverCallback, config map[string]interface{}) error {
	c := driverapi.Capability{
		DataScope: datastore.LocalScope,
	}
	d := &driver{
		networks: networkTable{},
	}
	d.initStore(config)

	return dc.RegisterDriver(macvlanType, d, c)
}

func (d *driver) NetworkAllocate(id string, option map[string]string, ipV4Data, ipV6Data []driverapi.IPAMData) (map[string]string, error) {
	return nil, types.NotImplementedErrorf("not implemented")
}

func (d *driver) NetworkFree(id string) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *driver) EndpointOperInfo(nid, eid string) (map[string]interface{}, error) {
	return make(map[string]interface{}, 0), nil
}

func (d *driver) Type() string {
	return macvlanType
}

func (d *driver) ProgramExternalConnectivity(nid, eid string, options map[string]interface{}) error {
	return nil
}

func (d *driver) RevokeExternalConnectivity(nid, eid string) error {
	return nil
}

// DiscoverNew is a notification for a new discovery event
func (d *driver) DiscoverNew(dType discoverapi.DiscoveryType, data interface{}) error {
	return nil
}

// DiscoverDelete is a notification for a discovery delete event
func (d *driver) DiscoverDelete(dType discoverapi.DiscoveryType, data interface{}) error {
	return nil
}

func (d *driver) EventNotify(etype driverapi.EventType, nid, tableName, key string, value []byte) {
}
