// Package protocol defines the protocol between the Fn Agent and the code
// running inside of a container. When an Fn Agent wants to perform a function
// call it needs to pass that call to a container over stdin. The call is
// encoded in one of the following protocols.
//
//	* Default I/O Format
//	* JSON I/O Format
//	* HTTP I/O Format
//
// For more information on the function formats see
// https://github.com/fnproject/fn/blob/master/docs/developers/function-format.md.
package protocol
