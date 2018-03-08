/*
	Interface between the Node Pool Manager and the Control Plane
*/

package controlplane

const CAPACITY_PER_RUNNER = 4096

type Runner struct {
	Id      string
	Address string
	// Other: certs etc here as managed and installed by CP
	Capacity int64
}

type ControlPlane interface {
	GetLBGRunners(lgbId string) ([]*Runner, error)
	ProvisionRunners(lgbId string, n int) (int, error)
	RemoveRunner(lbgId string, id string) error
}
