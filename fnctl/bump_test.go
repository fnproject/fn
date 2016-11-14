package main

import "testing"

func TestImageversion(t *testing.T) {
	type args struct {
		image string
	}
	tests := []struct {
		name     string
		args     args
		wantName string
		wantVer  string
	}{
		{"tag absent", args{"owner/imagename"}, "owner/imagename", initialVersion},
		{"non semver tag", args{"owner/imagename:tag"}, "owner/imagename", "0.0.1"},
		{"semver tag (M.m.p)", args{"owner/imagename:0.0.1"}, "owner/imagename", "0.0.1"},
		{"semver tag (vM.m.p)", args{"owner/imagename:v0.0.1"}, "owner/imagename", "0.0.1"},
	}
	for _, tt := range tests {
		gotName, gotVer := imageversion(tt.args.image)
		if gotName != tt.wantName {
			t.Errorf("%q. imageversion() gotName = %v, want %v", tt.name, gotName, tt.wantName)
		}
		if gotVer != tt.wantVer {
			t.Errorf("%q. imageversion() gotVer = %v, want %v", tt.name, gotVer, tt.wantVer)
		}
	}
}
