package models

import (
	"encoding/json"
	"fmt"
	"testing"
)

func checkStr(input string, expected MilliCPUs) error {
	var res MilliCPUs
	err := json.Unmarshal([]byte(input), &res)
	if err != nil {
		return err
	}
	if expected != res {
		return fmt.Errorf("mismatch %s != %s", res, expected)
	}
	return nil
}

func checkErr(input string) (MilliCPUs, error) {
	var res MilliCPUs
	err := json.Unmarshal([]byte(input), &res)
	if err != nil {
		return res, err
	}
	return res, nil
}

func TestMilliCPUsUnmarshal(t *testing.T) {

	err := checkStr("\"1.00\"", 1000)
	if err != nil {
		t.Fatal("failed: ", err)
	}

	err = checkStr("\"1\"", 1000)
	if err != nil {
		t.Fatal("failed: ", err)
	}

	err = checkStr("\"0\"", 0)
	if err != nil {
		t.Fatal("failed: ", err)
	}

	err = checkStr("\"00000\"", 0)
	if err != nil {
		t.Fatal("failed: ", err)
	}

	err = checkStr("\"+00000\"", 0)
	if err != nil {
		t.Fatal("failed: ", err)
	}

	err = checkStr("\"-00000\"", 0)
	if err != nil {
		t.Fatal("failed: ", err)
	}

	err = checkStr("\"0.01\"", 10)
	if err != nil {
		t.Fatal("failed: ", err)
	}

	tmp, err := checkErr("\"1000000000000000000000000\"")
	if err == nil {
		t.Fatal("failed, should get error, got: ", tmp)
	}

	// 0.2341 is too high of a precision for CPUs
	err = checkStr("\"0.2341\"", 234)
	if err != nil {
		t.Fatal("failed: ", err)
	}

	err = checkStr("\"1m\"", 1)
	if err != nil {
		t.Fatal("failed: ", err)
	}

	err = checkStr("\"1000m\"", 1000)
	if err != nil {
		t.Fatal("failed: ", err)
	}

	tmp, err = checkErr("\"-100\"")
	if err == nil {
		t.Fatal("failed, should get error, got: ", tmp)
	}

	tmp, err = checkErr("\"100000000000m\"")
	if err == nil {
		t.Fatal("failed, should get error, got: ", tmp)
	}

	err = checkStr("\".2\"", 200)
	if err != nil {
		t.Fatal("failed: ", err)
	}

	err = checkStr("\"100.2000\"", 100200)
	if err != nil {
		t.Fatal("failed: ", err)
	}

	tmp, err = checkErr("\"-0.20\"")
	if err == nil {
		t.Fatal("failed, should get error got: ", tmp)
	}
}
