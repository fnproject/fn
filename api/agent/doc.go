// Package agent defines the Agent interface and related concepts. An agent is
// an entity that knows how to execute an Fn function.
//
// The Agent Interface
//
// The Agent interface is the heart of this package. Agent exposes an api to
// create calls from various parameters and then execute those calls. An Agent
// has a few roles:
//	* manage the memory pool for a given server
//	* manage the container lifecycle for calls
//	* execute calls against containers
//	* invoke Start and End for each call appropriately
//
// For more information about how an agent executes a call see the
// documentation on the Agent interface.
//
// Variants
//
// There are two flavors of runner, the local Docker agent and a load-balancing
// agent. To create an agent that uses Docker containers to execute calls, use
// New().
//
// To create an agent that can load-balance across a pool of sub-agents, use
// NewLBAgent().
package agent
