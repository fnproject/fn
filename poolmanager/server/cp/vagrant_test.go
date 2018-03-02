package cp

import "testing"

func TestProvisionBox(t *testing.T) {
	vbox, err := NewVirtualBoxCP()
	if err != nil {
		t.Fatal(err)
	}
	err = vbox.provision()
	if err != nil {
		t.Fatal(err)
	}
}
