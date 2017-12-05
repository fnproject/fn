package cnmallocator

import (
	"fmt"
	"net"
	"testing"

	"github.com/docker/libnetwork/discoverapi"
	"github.com/docker/libnetwork/types"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/manager/allocator/networkallocator"
	"github.com/stretchr/testify/assert"
)

func newNetworkAllocator(t *testing.T) networkallocator.NetworkAllocator {
	na, err := New(nil)
	assert.NoError(t, err)
	assert.NotNil(t, na)
	return na
}

func TestNew(t *testing.T) {
	newNetworkAllocator(t)
}

func TestAllocateInvalidIPAM(t *testing.T) {
	na := newNetworkAllocator(t)
	n := &api.Network{
		ID: "testID",
		Spec: api.NetworkSpec{
			Annotations: api.Annotations{
				Name: "test",
			},
			DriverConfig: &api.Driver{},
			IPAM: &api.IPAMOptions{
				Driver: &api.Driver{
					Name: "invalidipam,",
				},
			},
		},
	}
	err := na.Allocate(n)
	assert.Error(t, err)
}

func TestAllocateInvalidDriver(t *testing.T) {
	na := newNetworkAllocator(t)
	n := &api.Network{
		ID: "testID",
		Spec: api.NetworkSpec{
			Annotations: api.Annotations{
				Name: "test",
			},
			DriverConfig: &api.Driver{
				Name: "invaliddriver",
			},
		},
	}

	err := na.Allocate(n)
	assert.Error(t, err)
}

func TestNetworkDoubleAllocate(t *testing.T) {
	na := newNetworkAllocator(t)
	n := &api.Network{
		ID: "testID",
		Spec: api.NetworkSpec{
			Annotations: api.Annotations{
				Name: "test",
			},
		},
	}

	err := na.Allocate(n)
	assert.NoError(t, err)

	err = na.Allocate(n)
	assert.Error(t, err)
}

func TestAllocateEmptyConfig(t *testing.T) {
	na1 := newNetworkAllocator(t)
	na2 := newNetworkAllocator(t)
	n1 := &api.Network{
		ID: "testID1",
		Spec: api.NetworkSpec{
			Annotations: api.Annotations{
				Name: "test1",
			},
		},
	}

	n2 := &api.Network{
		ID: "testID2",
		Spec: api.NetworkSpec{
			Annotations: api.Annotations{
				Name: "test2",
			},
		},
	}

	err := na1.Allocate(n1)
	assert.NoError(t, err)
	assert.NotEqual(t, n1.IPAM.Configs, nil)
	assert.Equal(t, len(n1.IPAM.Configs), 1)
	assert.Equal(t, n1.IPAM.Configs[0].Range, "")
	assert.Equal(t, len(n1.IPAM.Configs[0].Reserved), 0)

	_, subnet11, err := net.ParseCIDR(n1.IPAM.Configs[0].Subnet)
	assert.NoError(t, err)

	gwip11 := net.ParseIP(n1.IPAM.Configs[0].Gateway)
	assert.NotEqual(t, gwip11, nil)

	err = na1.Allocate(n2)
	assert.NoError(t, err)
	assert.NotEqual(t, n2.IPAM.Configs, nil)
	assert.Equal(t, len(n2.IPAM.Configs), 1)
	assert.Equal(t, n2.IPAM.Configs[0].Range, "")
	assert.Equal(t, len(n2.IPAM.Configs[0].Reserved), 0)

	_, subnet21, err := net.ParseCIDR(n2.IPAM.Configs[0].Subnet)
	assert.NoError(t, err)

	gwip21 := net.ParseIP(n2.IPAM.Configs[0].Gateway)
	assert.NotEqual(t, gwip21, nil)

	// Allocate n1 ans n2 with another allocator instance but in
	// intentionally reverse order.
	err = na2.Allocate(n2)
	assert.NoError(t, err)
	assert.NotEqual(t, n2.IPAM.Configs, nil)
	assert.Equal(t, len(n2.IPAM.Configs), 1)
	assert.Equal(t, n2.IPAM.Configs[0].Range, "")
	assert.Equal(t, len(n2.IPAM.Configs[0].Reserved), 0)

	_, subnet22, err := net.ParseCIDR(n2.IPAM.Configs[0].Subnet)
	assert.NoError(t, err)
	assert.Equal(t, subnet21, subnet22)

	gwip22 := net.ParseIP(n2.IPAM.Configs[0].Gateway)
	assert.Equal(t, gwip21, gwip22)

	err = na2.Allocate(n1)
	assert.NoError(t, err)
	assert.NotEqual(t, n1.IPAM.Configs, nil)
	assert.Equal(t, len(n1.IPAM.Configs), 1)
	assert.Equal(t, n1.IPAM.Configs[0].Range, "")
	assert.Equal(t, len(n1.IPAM.Configs[0].Reserved), 0)

	_, subnet12, err := net.ParseCIDR(n1.IPAM.Configs[0].Subnet)
	assert.NoError(t, err)
	assert.Equal(t, subnet11, subnet12)

	gwip12 := net.ParseIP(n1.IPAM.Configs[0].Gateway)
	assert.Equal(t, gwip11, gwip12)
}

func TestAllocateWithOneSubnet(t *testing.T) {
	na := newNetworkAllocator(t)
	n := &api.Network{
		ID: "testID",
		Spec: api.NetworkSpec{
			Annotations: api.Annotations{
				Name: "test",
			},
			DriverConfig: &api.Driver{},
			IPAM: &api.IPAMOptions{
				Driver: &api.Driver{},
				Configs: []*api.IPAMConfig{
					{
						Subnet: "192.168.1.0/24",
					},
				},
			},
		},
	}

	err := na.Allocate(n)
	assert.NoError(t, err)
	assert.Equal(t, len(n.IPAM.Configs), 1)
	assert.Equal(t, n.IPAM.Configs[0].Range, "")
	assert.Equal(t, len(n.IPAM.Configs[0].Reserved), 0)
	assert.Equal(t, n.IPAM.Configs[0].Subnet, "192.168.1.0/24")

	ip := net.ParseIP(n.IPAM.Configs[0].Gateway)
	assert.NotEqual(t, ip, nil)
}

func TestAllocateWithOneSubnetGateway(t *testing.T) {
	na := newNetworkAllocator(t)
	n := &api.Network{
		ID: "testID",
		Spec: api.NetworkSpec{
			Annotations: api.Annotations{
				Name: "test",
			},
			DriverConfig: &api.Driver{},
			IPAM: &api.IPAMOptions{
				Driver: &api.Driver{},
				Configs: []*api.IPAMConfig{
					{
						Subnet:  "192.168.1.0/24",
						Gateway: "192.168.1.1",
					},
				},
			},
		},
	}

	err := na.Allocate(n)
	assert.NoError(t, err)
	assert.Equal(t, len(n.IPAM.Configs), 1)
	assert.Equal(t, n.IPAM.Configs[0].Range, "")
	assert.Equal(t, len(n.IPAM.Configs[0].Reserved), 0)
	assert.Equal(t, n.IPAM.Configs[0].Subnet, "192.168.1.0/24")
	assert.Equal(t, n.IPAM.Configs[0].Gateway, "192.168.1.1")
}

func TestAllocateWithOneSubnetInvalidGateway(t *testing.T) {
	na := newNetworkAllocator(t)
	n := &api.Network{
		ID: "testID",
		Spec: api.NetworkSpec{
			Annotations: api.Annotations{
				Name: "test",
			},
			DriverConfig: &api.Driver{},
			IPAM: &api.IPAMOptions{
				Driver: &api.Driver{},
				Configs: []*api.IPAMConfig{
					{
						Subnet:  "192.168.1.0/24",
						Gateway: "192.168.2.1",
					},
				},
			},
		},
	}

	err := na.Allocate(n)
	assert.Error(t, err)
}

func TestAllocateWithInvalidSubnet(t *testing.T) {
	na := newNetworkAllocator(t)
	n := &api.Network{
		ID: "testID",
		Spec: api.NetworkSpec{
			Annotations: api.Annotations{
				Name: "test",
			},
			DriverConfig: &api.Driver{},
			IPAM: &api.IPAMOptions{
				Driver: &api.Driver{},
				Configs: []*api.IPAMConfig{
					{
						Subnet: "1.1.1.1/32",
					},
				},
			},
		},
	}

	err := na.Allocate(n)
	assert.Error(t, err)
}

func TestAllocateWithTwoSubnetsNoGateway(t *testing.T) {
	na := newNetworkAllocator(t)
	n := &api.Network{
		ID: "testID",
		Spec: api.NetworkSpec{
			Annotations: api.Annotations{
				Name: "test",
			},
			DriverConfig: &api.Driver{},
			IPAM: &api.IPAMOptions{
				Driver: &api.Driver{},
				Configs: []*api.IPAMConfig{
					{
						Subnet: "192.168.1.0/24",
					},
					{
						Subnet: "192.168.2.0/24",
					},
				},
			},
		},
	}

	err := na.Allocate(n)
	assert.NoError(t, err)
	assert.Equal(t, len(n.IPAM.Configs), 2)
	assert.Equal(t, n.IPAM.Configs[0].Range, "")
	assert.Equal(t, len(n.IPAM.Configs[0].Reserved), 0)
	assert.Equal(t, n.IPAM.Configs[0].Subnet, "192.168.1.0/24")
	assert.Equal(t, n.IPAM.Configs[1].Range, "")
	assert.Equal(t, len(n.IPAM.Configs[1].Reserved), 0)
	assert.Equal(t, n.IPAM.Configs[1].Subnet, "192.168.2.0/24")

	ip := net.ParseIP(n.IPAM.Configs[0].Gateway)
	assert.NotEqual(t, ip, nil)
	ip = net.ParseIP(n.IPAM.Configs[1].Gateway)
	assert.NotEqual(t, ip, nil)
}

func TestFree(t *testing.T) {
	na := newNetworkAllocator(t)
	n := &api.Network{
		ID: "testID",
		Spec: api.NetworkSpec{
			Annotations: api.Annotations{
				Name: "test",
			},
			DriverConfig: &api.Driver{},
			IPAM: &api.IPAMOptions{
				Driver: &api.Driver{},
				Configs: []*api.IPAMConfig{
					{
						Subnet:  "192.168.1.0/24",
						Gateway: "192.168.1.1",
					},
				},
			},
		},
	}

	err := na.Allocate(n)
	assert.NoError(t, err)

	err = na.Deallocate(n)
	assert.NoError(t, err)

	// Reallocate again to make sure it succeeds.
	err = na.Allocate(n)
	assert.NoError(t, err)
}

func TestAllocateTaskFree(t *testing.T) {
	na1 := newNetworkAllocator(t)
	na2 := newNetworkAllocator(t)
	n1 := &api.Network{
		ID: "testID1",
		Spec: api.NetworkSpec{
			Annotations: api.Annotations{
				Name: "test1",
			},
			DriverConfig: &api.Driver{},
			IPAM: &api.IPAMOptions{
				Driver: &api.Driver{},
				Configs: []*api.IPAMConfig{
					{
						Subnet:  "192.168.1.0/24",
						Gateway: "192.168.1.1",
					},
				},
			},
		},
	}

	n2 := &api.Network{
		ID: "testID2",
		Spec: api.NetworkSpec{
			Annotations: api.Annotations{
				Name: "test2",
			},
			DriverConfig: &api.Driver{},
			IPAM: &api.IPAMOptions{
				Driver: &api.Driver{},
				Configs: []*api.IPAMConfig{
					{
						Subnet:  "192.168.2.0/24",
						Gateway: "192.168.2.1",
					},
				},
			},
		},
	}

	task1 := &api.Task{
		Networks: []*api.NetworkAttachment{
			{
				Network: n1,
			},
			{
				Network: n2,
			},
		},
	}

	task2 := &api.Task{
		Networks: []*api.NetworkAttachment{
			{
				Network: n1,
			},
			{
				Network: n2,
			},
		},
	}

	err := na1.Allocate(n1)
	assert.NoError(t, err)

	err = na1.Allocate(n2)
	assert.NoError(t, err)

	err = na1.AllocateTask(task1)
	assert.NoError(t, err)
	assert.Equal(t, len(task1.Networks[0].Addresses), 1)
	assert.Equal(t, len(task1.Networks[1].Addresses), 1)

	_, subnet1, _ := net.ParseCIDR("192.168.1.0/24")
	_, subnet2, _ := net.ParseCIDR("192.168.2.0/24")

	// variable coding: network/task/allocator
	ip111, _, err := net.ParseCIDR(task1.Networks[0].Addresses[0])
	assert.NoError(t, err)

	ip211, _, err := net.ParseCIDR(task1.Networks[1].Addresses[0])
	assert.NoError(t, err)

	assert.Equal(t, subnet1.Contains(ip111), true)
	assert.Equal(t, subnet2.Contains(ip211), true)

	err = na1.AllocateTask(task2)
	assert.NoError(t, err)
	assert.Equal(t, len(task2.Networks[0].Addresses), 1)
	assert.Equal(t, len(task2.Networks[1].Addresses), 1)

	ip121, _, err := net.ParseCIDR(task2.Networks[0].Addresses[0])
	assert.NoError(t, err)

	ip221, _, err := net.ParseCIDR(task2.Networks[1].Addresses[0])
	assert.NoError(t, err)

	assert.Equal(t, subnet1.Contains(ip121), true)
	assert.Equal(t, subnet2.Contains(ip221), true)

	// Now allocate the same the same tasks in a second allocator
	// but intentionally in reverse order.
	err = na2.Allocate(n1)
	assert.NoError(t, err)

	err = na2.Allocate(n2)
	assert.NoError(t, err)

	err = na2.AllocateTask(task2)
	assert.NoError(t, err)
	assert.Equal(t, len(task2.Networks[0].Addresses), 1)
	assert.Equal(t, len(task2.Networks[1].Addresses), 1)

	ip122, _, err := net.ParseCIDR(task2.Networks[0].Addresses[0])
	assert.NoError(t, err)

	ip222, _, err := net.ParseCIDR(task2.Networks[1].Addresses[0])
	assert.NoError(t, err)

	assert.Equal(t, subnet1.Contains(ip122), true)
	assert.Equal(t, subnet2.Contains(ip222), true)
	assert.Equal(t, ip121, ip122)
	assert.Equal(t, ip221, ip222)

	err = na2.AllocateTask(task1)
	assert.NoError(t, err)
	assert.Equal(t, len(task1.Networks[0].Addresses), 1)
	assert.Equal(t, len(task1.Networks[1].Addresses), 1)

	ip112, _, err := net.ParseCIDR(task1.Networks[0].Addresses[0])
	assert.NoError(t, err)

	ip212, _, err := net.ParseCIDR(task1.Networks[1].Addresses[0])
	assert.NoError(t, err)

	assert.Equal(t, subnet1.Contains(ip112), true)
	assert.Equal(t, subnet2.Contains(ip212), true)
	assert.Equal(t, ip111, ip112)
	assert.Equal(t, ip211, ip212)

	// Deallocate task
	err = na1.DeallocateTask(task1)
	assert.NoError(t, err)
	assert.Equal(t, len(task1.Networks[0].Addresses), 0)
	assert.Equal(t, len(task1.Networks[1].Addresses), 0)

	// Try allocation after free
	err = na1.AllocateTask(task1)
	assert.NoError(t, err)
	assert.Equal(t, len(task1.Networks[0].Addresses), 1)
	assert.Equal(t, len(task1.Networks[1].Addresses), 1)

	ip111, _, err = net.ParseCIDR(task1.Networks[0].Addresses[0])
	assert.NoError(t, err)

	ip211, _, err = net.ParseCIDR(task1.Networks[1].Addresses[0])
	assert.NoError(t, err)

	assert.Equal(t, subnet1.Contains(ip111), true)
	assert.Equal(t, subnet2.Contains(ip211), true)

	err = na1.DeallocateTask(task1)
	assert.NoError(t, err)
	assert.Equal(t, len(task1.Networks[0].Addresses), 0)
	assert.Equal(t, len(task1.Networks[1].Addresses), 0)

	// Try to free endpoints on an already freed task
	err = na1.DeallocateTask(task1)
	assert.NoError(t, err)
}

func TestAllocateService(t *testing.T) {
	na := newNetworkAllocator(t)
	n := &api.Network{
		ID: "testID",
		Spec: api.NetworkSpec{
			Annotations: api.Annotations{
				Name: "test",
			},
		},
	}

	s := &api.Service{
		ID: "testID1",
		Spec: api.ServiceSpec{
			Task: api.TaskSpec{
				Networks: []*api.NetworkAttachmentConfig{
					{
						Target: "testID",
					},
				},
			},
			Endpoint: &api.EndpointSpec{
				Ports: []*api.PortConfig{
					{
						Name:       "http",
						TargetPort: 80,
					},
					{
						Name:       "https",
						TargetPort: 443,
					},
				},
			},
		},
	}

	err := na.Allocate(n)
	assert.NoError(t, err)
	assert.NotEqual(t, n.IPAM.Configs, nil)
	assert.Equal(t, len(n.IPAM.Configs), 1)
	assert.Equal(t, n.IPAM.Configs[0].Range, "")
	assert.Equal(t, len(n.IPAM.Configs[0].Reserved), 0)

	_, subnet, err := net.ParseCIDR(n.IPAM.Configs[0].Subnet)
	assert.NoError(t, err)

	gwip := net.ParseIP(n.IPAM.Configs[0].Gateway)
	assert.NotEqual(t, gwip, nil)

	err = na.AllocateService(s)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(s.Endpoint.Ports))
	assert.True(t, s.Endpoint.Ports[0].PublishedPort >= dynamicPortStart &&
		s.Endpoint.Ports[0].PublishedPort <= dynamicPortEnd)
	assert.True(t, s.Endpoint.Ports[1].PublishedPort >= dynamicPortStart &&
		s.Endpoint.Ports[1].PublishedPort <= dynamicPortEnd)

	assert.Equal(t, 1, len(s.Endpoint.VirtualIPs))

	assert.Equal(t, s.Endpoint.Spec, s.Spec.Endpoint)

	ip, _, err := net.ParseCIDR(s.Endpoint.VirtualIPs[0].Addr)
	assert.NoError(t, err)

	assert.Equal(t, true, subnet.Contains(ip))
}

func TestAllocateServiceUserDefinedPorts(t *testing.T) {
	na := newNetworkAllocator(t)
	s := &api.Service{
		ID: "testID1",
		Spec: api.ServiceSpec{
			Endpoint: &api.EndpointSpec{
				Ports: []*api.PortConfig{
					{
						Name:          "some_tcp",
						TargetPort:    1234,
						PublishedPort: 1234,
					},
					{
						Name:          "some_udp",
						TargetPort:    1234,
						PublishedPort: 1234,
						Protocol:      api.ProtocolUDP,
					},
				},
			},
		},
	}

	err := na.AllocateService(s)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(s.Endpoint.Ports))
	assert.Equal(t, uint32(1234), s.Endpoint.Ports[0].PublishedPort)
	assert.Equal(t, uint32(1234), s.Endpoint.Ports[1].PublishedPort)
}

func TestAllocateServiceConflictingUserDefinedPorts(t *testing.T) {
	na := newNetworkAllocator(t)
	s := &api.Service{
		ID: "testID1",
		Spec: api.ServiceSpec{
			Endpoint: &api.EndpointSpec{
				Ports: []*api.PortConfig{
					{
						Name:          "some_tcp",
						TargetPort:    1234,
						PublishedPort: 1234,
					},
					{
						Name:          "some_other_tcp",
						TargetPort:    1234,
						PublishedPort: 1234,
					},
				},
			},
		},
	}

	err := na.AllocateService(s)
	assert.Error(t, err)
}

func TestDeallocateServiceAllocate(t *testing.T) {
	na := newNetworkAllocator(t)
	s := &api.Service{
		ID: "testID1",
		Spec: api.ServiceSpec{
			Endpoint: &api.EndpointSpec{
				Ports: []*api.PortConfig{
					{
						Name:          "some_tcp",
						TargetPort:    1234,
						PublishedPort: 1234,
					},
				},
			},
		},
	}

	err := na.AllocateService(s)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(s.Endpoint.Ports))
	assert.Equal(t, uint32(1234), s.Endpoint.Ports[0].PublishedPort)

	err = na.DeallocateService(s)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(s.Endpoint.Ports))
	// Allocate again.
	err = na.AllocateService(s)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(s.Endpoint.Ports))
	assert.Equal(t, uint32(1234), s.Endpoint.Ports[0].PublishedPort)
}

func TestDeallocateServiceAllocateIngressMode(t *testing.T) {
	na := newNetworkAllocator(t)

	n := &api.Network{
		ID: "testNetID1",
		Spec: api.NetworkSpec{
			Annotations: api.Annotations{
				Name: "test",
			},
			Ingress: true,
		},
	}

	err := na.Allocate(n)
	assert.NoError(t, err)

	s := &api.Service{
		ID: "testID1",
		Spec: api.ServiceSpec{
			Endpoint: &api.EndpointSpec{
				Ports: []*api.PortConfig{
					{
						Name:          "some_tcp",
						TargetPort:    1234,
						PublishedPort: 1234,
						PublishMode:   api.PublishModeIngress,
					},
				},
			},
		},
		Endpoint: &api.Endpoint{},
	}

	s.Endpoint.VirtualIPs = append(s.Endpoint.VirtualIPs,
		&api.Endpoint_VirtualIP{NetworkID: n.ID})

	err = na.AllocateService(s)
	assert.NoError(t, err)
	assert.Len(t, s.Endpoint.Ports, 1)
	assert.Equal(t, uint32(1234), s.Endpoint.Ports[0].PublishedPort)
	assert.Len(t, s.Endpoint.VirtualIPs, 1)

	err = na.DeallocateService(s)
	assert.NoError(t, err)
	assert.Len(t, s.Endpoint.Ports, 0)
	assert.Len(t, s.Endpoint.VirtualIPs, 0)
	// Allocate again.
	s.Endpoint.VirtualIPs = append(s.Endpoint.VirtualIPs,
		&api.Endpoint_VirtualIP{NetworkID: n.ID})

	err = na.AllocateService(s)
	assert.NoError(t, err)
	assert.Len(t, s.Endpoint.Ports, 1)
	assert.Equal(t, uint32(1234), s.Endpoint.Ports[0].PublishedPort)
	assert.Len(t, s.Endpoint.VirtualIPs, 1)
}

func TestServiceAddRemovePortsIngressMode(t *testing.T) {
	na := newNetworkAllocator(t)

	n := &api.Network{
		ID: "testNetID1",
		Spec: api.NetworkSpec{
			Annotations: api.Annotations{
				Name: "test",
			},
			Ingress: true,
		},
	}

	err := na.Allocate(n)
	assert.NoError(t, err)

	s := &api.Service{
		ID: "testID1",
		Spec: api.ServiceSpec{
			Endpoint: &api.EndpointSpec{
				Ports: []*api.PortConfig{
					{
						Name:          "some_tcp",
						TargetPort:    1234,
						PublishedPort: 1234,
						PublishMode:   api.PublishModeIngress,
					},
				},
			},
		},
		Endpoint: &api.Endpoint{},
	}

	s.Endpoint.VirtualIPs = append(s.Endpoint.VirtualIPs,
		&api.Endpoint_VirtualIP{NetworkID: n.ID})

	err = na.AllocateService(s)
	assert.NoError(t, err)
	assert.Len(t, s.Endpoint.Ports, 1)
	assert.Equal(t, uint32(1234), s.Endpoint.Ports[0].PublishedPort)
	assert.Len(t, s.Endpoint.VirtualIPs, 1)
	allocatedVIP := s.Endpoint.VirtualIPs[0].Addr

	//Unpublish port
	s.Spec.Endpoint.Ports = s.Spec.Endpoint.Ports[:0]
	err = na.AllocateService(s)
	assert.NoError(t, err)
	assert.Len(t, s.Endpoint.Ports, 0)
	assert.Len(t, s.Endpoint.VirtualIPs, 0)

	// Publish port again and ensure VIP is not the same that was deallocated.
	// Since IP allocation is serial we should  receive the next available IP.
	s.Spec.Endpoint.Ports = append(s.Spec.Endpoint.Ports, &api.PortConfig{Name: "some_tcp",
		TargetPort:    1234,
		PublishedPort: 1234,
		PublishMode:   api.PublishModeIngress,
	})
	s.Endpoint.VirtualIPs = append(s.Endpoint.VirtualIPs,
		&api.Endpoint_VirtualIP{NetworkID: n.ID})
	err = na.AllocateService(s)
	assert.NoError(t, err)
	assert.Len(t, s.Endpoint.Ports, 1)
	assert.Equal(t, uint32(1234), s.Endpoint.Ports[0].PublishedPort)
	assert.Len(t, s.Endpoint.VirtualIPs, 1)
	assert.NotEqual(t, allocatedVIP, s.Endpoint.VirtualIPs[0].Addr)
}

func TestServiceUpdate(t *testing.T) {
	na1 := newNetworkAllocator(t)
	na2 := newNetworkAllocator(t)
	s := &api.Service{
		ID: "testID1",
		Spec: api.ServiceSpec{
			Endpoint: &api.EndpointSpec{
				Ports: []*api.PortConfig{
					{
						Name:          "some_tcp",
						TargetPort:    1234,
						PublishedPort: 1234,
					},
					{
						Name:          "some_other_tcp",
						TargetPort:    1235,
						PublishedPort: 0,
					},
				},
			},
		},
	}

	err := na1.AllocateService(s)
	assert.NoError(t, err)
	assert.True(t, na1.IsServiceAllocated(s))
	assert.Equal(t, 2, len(s.Endpoint.Ports))
	assert.Equal(t, uint32(1234), s.Endpoint.Ports[0].PublishedPort)
	assert.NotEqual(t, 0, s.Endpoint.Ports[1].PublishedPort)

	// Cache the secode node port
	allocatedPort := s.Endpoint.Ports[1].PublishedPort

	// Now allocate the same service in another allocator instance
	err = na2.AllocateService(s)
	assert.NoError(t, err)
	assert.True(t, na2.IsServiceAllocated(s))
	assert.Equal(t, 2, len(s.Endpoint.Ports))
	assert.Equal(t, uint32(1234), s.Endpoint.Ports[0].PublishedPort)
	// Make sure we got the same port
	assert.Equal(t, allocatedPort, s.Endpoint.Ports[1].PublishedPort)

	s.Spec.Endpoint.Ports[1].PublishedPort = 1235
	assert.False(t, na1.IsServiceAllocated(s))

	err = na1.AllocateService(s)
	assert.NoError(t, err)
	assert.True(t, na1.IsServiceAllocated(s))
	assert.Equal(t, 2, len(s.Endpoint.Ports))
	assert.Equal(t, uint32(1234), s.Endpoint.Ports[0].PublishedPort)
	assert.Equal(t, uint32(1235), s.Endpoint.Ports[1].PublishedPort)
}

func TestServiceNetworkUpdate(t *testing.T) {
	na := newNetworkAllocator(t)

	n1 := &api.Network{
		ID: "testID1",
		Spec: api.NetworkSpec{
			Annotations: api.Annotations{
				Name: "test",
			},
		},
	}

	n2 := &api.Network{
		ID: "testID2",
		Spec: api.NetworkSpec{
			Annotations: api.Annotations{
				Name: "test2",
			},
		},
	}

	//Allocate both networks
	err := na.Allocate(n1)
	assert.NoError(t, err)

	err = na.Allocate(n2)
	assert.NoError(t, err)

	//Attach a network to a service spec nd allocate a service
	s := &api.Service{
		ID: "testID1",
		Spec: api.ServiceSpec{
			Task: api.TaskSpec{
				Networks: []*api.NetworkAttachmentConfig{
					{
						Target: "testID1",
					},
				},
			},
			Endpoint: &api.EndpointSpec{
				Mode: api.ResolutionModeVirtualIP,
			},
		},
	}

	err = na.AllocateService(s)
	assert.NoError(t, err)
	assert.True(t, na.IsServiceAllocated(s))
	assert.Len(t, s.Endpoint.VirtualIPs, 1)

	// Now update the same service with another network
	s.Spec.Task.Networks = append(s.Spec.Task.Networks, &api.NetworkAttachmentConfig{Target: "testID2"})

	assert.False(t, na.IsServiceAllocated(s))
	err = na.AllocateService(s)
	assert.NoError(t, err)

	assert.True(t, na.IsServiceAllocated(s))
	assert.Len(t, s.Endpoint.VirtualIPs, 2)

	s.Spec.Task.Networks = s.Spec.Task.Networks[:1]

	//Check if service needs update and allocate with updated service spec
	assert.False(t, na.IsServiceAllocated(s))

	err = na.AllocateService(s)
	assert.NoError(t, err)
	assert.True(t, na.IsServiceAllocated(s))
	assert.Len(t, s.Endpoint.VirtualIPs, 1)

	s.Spec.Task.Networks = s.Spec.Task.Networks[:0]
	//Check if service needs update with all the networks removed and allocate with updated service spec
	assert.False(t, na.IsServiceAllocated(s))

	err = na.AllocateService(s)
	assert.NoError(t, err)
	assert.True(t, na.IsServiceAllocated(s))
	assert.Len(t, s.Endpoint.VirtualIPs, 0)

	//Attach a network and allocate service
	s.Spec.Task.Networks = append(s.Spec.Task.Networks, &api.NetworkAttachmentConfig{Target: "testID2"})
	assert.False(t, na.IsServiceAllocated(s))

	err = na.AllocateService(s)
	assert.NoError(t, err)

	assert.True(t, na.IsServiceAllocated(s))
	assert.Len(t, s.Endpoint.VirtualIPs, 1)

}

type mockIpam struct {
	actualIpamOptions map[string]string
}

func (a *mockIpam) GetDefaultAddressSpaces() (string, string, error) {
	return "defaultAS", "defaultAS", nil
}

func (a *mockIpam) RequestPool(addressSpace, pool, subPool string, options map[string]string, v6 bool) (string, *net.IPNet, map[string]string, error) {
	a.actualIpamOptions = options

	poolCidr, _ := types.ParseCIDR(pool)
	return fmt.Sprintf("%s/%s", "defaultAS", pool), poolCidr, nil, nil
}

func (a *mockIpam) ReleasePool(poolID string) error {
	return nil
}

func (a *mockIpam) RequestAddress(poolID string, ip net.IP, opts map[string]string) (*net.IPNet, map[string]string, error) {
	return nil, nil, nil
}

func (a *mockIpam) ReleaseAddress(poolID string, ip net.IP) error {
	return nil
}

func (a *mockIpam) DiscoverNew(dType discoverapi.DiscoveryType, data interface{}) error {
	return nil
}

func (a *mockIpam) DiscoverDelete(dType discoverapi.DiscoveryType, data interface{}) error {
	return nil
}

func (a *mockIpam) IsBuiltIn() bool {
	return true
}

func TestCorrectlyPassIPAMOptions(t *testing.T) {
	var err error
	expectedIpamOptions := map[string]string{"network-name": "freddie"}

	na := newNetworkAllocator(t)
	ipamDriver := &mockIpam{}

	err = na.(*cnmNetworkAllocator).drvRegistry.RegisterIpamDriver("mockipam", ipamDriver)
	assert.NoError(t, err)

	n := &api.Network{
		ID: "testID",
		Spec: api.NetworkSpec{
			Annotations: api.Annotations{
				Name: "test",
			},
			DriverConfig: &api.Driver{},
			IPAM: &api.IPAMOptions{
				Driver: &api.Driver{
					Name:    "mockipam",
					Options: expectedIpamOptions,
				},
				Configs: []*api.IPAMConfig{
					{
						Subnet:  "192.168.1.0/24",
						Gateway: "192.168.1.1",
					},
				},
			},
		},
	}
	err = na.Allocate(n)

	assert.Equal(t, expectedIpamOptions, ipamDriver.actualIpamOptions)
	assert.NoError(t, err)
}
