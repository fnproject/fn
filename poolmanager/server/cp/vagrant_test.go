package cp

import "testing"

func TestProvisionBox(t *testing.T) {
	vbox, err := NewVirtualBoxCP()
	if err != nil {
		t.Fatal(err)
	}
	_, err = vbox.provision()
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetAddr(t *testing.T) {
	_, err := getNodeAddr("fn-vagrant")
	if err != nil {
		t.Fatal(err)
	}
}
