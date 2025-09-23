// Package drivers is intended as a general purpose container abstraction
// library. It abstracts across the differences between different container
// runtimes (e.g. Docker, Rkt, etc.) and provides utlities and data types that
// are common across all runtimes.
//
// # Docker Driver
//
// The docker driver runs functions as Docker containers.
//
// # Mock Driver
//
// The mock driver pretends to run functions but doesn't actually run them. This
// is for testing only.
package drivers
