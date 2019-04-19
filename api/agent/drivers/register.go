package drivers

import (
	"fmt"
)

type DriverFunc func(config Config) (Driver, error)

var drivers = make(map[string]DriverFunc)

// Register adds  a container driver by name to this process
func Register(name string, driverFunc DriverFunc) {
	drivers[name] = driverFunc
}

// New Instantiates a driver by name
func New(driverName string, config Config) (Driver, error) {
	driverFunc, ok := drivers[driverName]

	if !ok {
		return nil, fmt.Errorf("agent driver \"%s\" is not registered", driverName)
	}
	return driverFunc(config)
}
