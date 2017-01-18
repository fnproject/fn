# Triggers

Triggers are integrations that you can use in other systems to fire off functions in IronFunctions.

## OpenStack

### Requirements

1. OpenStack or DevStack environment.

    * OS: Ubuntu 16.04 LTS or newer
    * Kernel: 4.7 or newer with overlay2 or aufs module
    * Docker: 1.12 or newer

2. [Picasso](https://github.com/openstack/picasso) - Picasso provides an OpenStack API and Keystone authentication layer on top of IronFunctions.
Please refer to the [Picasso on DevStack](https://github.com/openstack/picasso/blob/master/devstack/README.md) guide for setup instructions.

### Examples

* [Alarm event triggers from Telemetry and Aodh](https://github.com/openstack/picasso/blob/master/examples/openstack-alarms/README.md)
* Swift notifications - *Status: On roadmap*
    * [Blueprint](https://blueprints.launchpad.net/picasso/+spec/swift-notifications)