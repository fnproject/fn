#Vagrant testing

We've created a control plane interface for local development using vagrant as a backend. It interacts with minikube, where the rest of the components of the fn project are expected to run for local development.

##Getting it working

In order to create virtual machines you're going to want to configure the minikube and the hosts provided to share a network adapter. If you haven't already, [download the binary](https://github.com/kubernetes/minikube) and run `minikube start --vm-provider=virtualbox`. This should configure a new virtual box host only network called `vboxnet0`. From there, you should be able to run thsis code as is to start VMs backed by virtual box.

##Issues

Occasionally, you may run into an issue with the DHCP server the virtual box configures and will not be able to start a server.

If you see a message like this when running `vagrant up`:

```
A host only network interface you're attempting to configure via DHCP
already has a conflicting host only adapter with DHCP enabled. The
DHCP on this adapter is incompatible with the DHCP settings. Two
host only network interfaces are not allowed to overlap, and each
host only network interface can have only one DHCP server. Please
reconfigure your host only network or remove the virtual machine
using the other host only network.
```

Running the following command should clear all of these collision problems:

`VBoxManage dhcpserver remove --netname HostInterfaceNetworking-vboxnet0`

##Support

This is only intended to be used to test distributed components any use further than this will not be supported.
