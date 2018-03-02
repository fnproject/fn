package cp

import (
	"errors"
	"os/exec"
)

var whichVBox *exec.Cmd

func init() {
	whichVBox = exec.Command("which", "vbox")
}

type VirtualBoxCP struct{}

func NewVirtualBoxCP() (*VirtualBoxCP, error) {
	if err := whichVBox.Run(); err != nil {
		return nil, err
	}
	return &VirtualBoxCP{}, nil
}

func (v *VirtualBoxCP) provision() error {
	return errors.New("Not implemented yet")
}

func (v *VirtualBoxCP) GetLBGRunners(lgbId string) ([]*Runner, error) {
	return nil, errors.New("Not done")

}

func (v *VirtualBoxCP) ProvisionRunners(lgbId string, n int) (int, error) {
	return -1, errors.New("Not done")
}

func (v *VirtualBoxCP) RemoveRunner(lbgId string, id string) error {
	return errors.New("Not done")
}
