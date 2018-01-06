package models

import (
	"fmt"
	"testing"
)

func checkStr(input, expected string) error {
	res, err := sanitizeCPUs(input)
	if err != nil {
		return err
	}
	if expected != res {
		return fmt.Errorf("mismatch %s != %s", res, expected)
	}
	return nil
}

func TestRouteCPUSanitize(t *testing.T) {

	err := checkStr("1.00", "1.00")
	if err != nil {
		t.Fatal("failed: ", err)
	}

	err = checkStr("1", "1.00")
	if err != nil {
		t.Fatal("failed: ", err)
	}

	err = checkStr("", "")
	if err != nil {
		t.Fatal("failed: ", err)
	}

	err = checkStr("0", "")
	if err != nil {
		t.Fatal("failed: ", err)
	}

	err = checkStr("00000", "")
	if err != nil {
		t.Fatal("failed: ", err)
	}

	err = checkStr("+00000", "")
	if err != nil {
		t.Fatal("failed: ", err)
	}

	err = checkStr("-00000", "")
	if err != nil {
		t.Fatal("failed: ", err)
	}

	err = checkStr("00000000000000000000", "")
	if err != nil {
		t.Fatal("failed: ", err)
	}

	tmp, err := sanitizeCPUs("1000000000000000000000000")
	if err == nil {
		t.Fatal("failed, should get error, got: ", tmp)
	}

	// 0.234 is too high of a precision for CPUs
	tmp, err = sanitizeCPUs("0.234")
	if err == nil {
		t.Fatal("failed, should get error got: ", tmp)
	}

	err = checkStr(".2", "0.20")
	if err != nil {
		t.Fatal("failed: ", err)
	}

	err = checkStr("100.2000", "100.20")
	if err != nil {
		t.Fatal("failed: ", err)
	}

	tmp, err = sanitizeCPUs("-0.20")
	if err == nil {
		t.Fatal("failed, should get error got: ", tmp)
	}
}
