package provider

import (
	"sort"
	"testing"

	"github.com/ritajhq/terraform-provider-dockercompose/internal/docker"
)

// ============================================================
// Unit Tests for profile_lifecycle.go: profile-based service activation/diffing
// ============================================================

func testComposeFileWithProfiles() *docker.ComposeFile {
	return &docker.ComposeFile{
		Services: map[string]*docker.ServiceConfig{
			"web":   {Image: "nginx:latest"}, // no profiles: always active
			"debug": {Image: "debugger:latest", Profiles: []string{"debug"}},
			"admin": {Image: "admin:latest", Profiles: []string{"debug", "admin"}},
		},
	}
}

func TestServicesActiveForProfilesNoProfilesActive(t *testing.T) {
	cf := testComposeFileWithProfiles()
	active := servicesActiveForProfiles(cf, nil)

	if !active["web"] {
		t.Error("web (no profiles) should always be active")
	}
	if active["debug"] {
		t.Error("debug should not be active when no profiles are active")
	}
	if active["admin"] {
		t.Error("admin should not be active when no profiles are active")
	}
}

func TestServicesActiveForProfilesOneActive(t *testing.T) {
	cf := testComposeFileWithProfiles()
	active := servicesActiveForProfiles(cf, []string{"debug"})

	if !active["web"] {
		t.Error("web should always be active")
	}
	if !active["debug"] {
		t.Error("debug should be active when 'debug' profile is active")
	}
	if !active["admin"] {
		t.Error("admin should be active since it includes the 'debug' profile")
	}
}

func TestServicesActiveForProfilesUnrelatedProfile(t *testing.T) {
	cf := testComposeFileWithProfiles()
	active := servicesActiveForProfiles(cf, []string{"unrelated"})

	if active["debug"] {
		t.Error("debug should not be active for an unrelated active profile")
	}
	if active["admin"] {
		t.Error("admin should not be active for an unrelated active profile")
	}
}

func TestServicesDroppedByProfilesNoneDropped(t *testing.T) {
	cf := testComposeFileWithProfiles()
	dropped := servicesDroppedByProfiles(cf, []string{"debug"}, []string{"debug", "admin"})

	if len(dropped) != 0 {
		t.Errorf("servicesDroppedByProfiles() = %v, want none dropped when adding a profile", dropped)
	}
}

func TestServicesDroppedByProfilesSomeDropped(t *testing.T) {
	cf := testComposeFileWithProfiles()
	dropped := servicesDroppedByProfiles(cf, []string{"debug", "admin"}, []string{"admin"})

	sort.Strings(dropped)

	found := false
	for _, name := range dropped {
		if name == "debug" {
			found = true
		}
	}
	if !found {
		t.Errorf("servicesDroppedByProfiles() = %v, want 'debug' to be dropped when 'debug' profile is removed", dropped)
	}
	// admin service has profiles [debug, admin]; "admin" is still active, so admin should NOT be dropped.
	for _, name := range dropped {
		if name == "admin" {
			t.Errorf("servicesDroppedByProfiles() = %v, admin should remain active via the 'admin' profile", dropped)
		}
	}
}

func TestServicesDroppedByProfilesAllDropped(t *testing.T) {
	cf := testComposeFileWithProfiles()
	dropped := servicesDroppedByProfiles(cf, []string{"debug", "admin"}, nil)

	sort.Strings(dropped)
	if len(dropped) != 2 || dropped[0] != "admin" || dropped[1] != "debug" {
		t.Errorf("servicesDroppedByProfiles() = %v, want [admin debug] when all profiles are cleared", dropped)
	}
}

func TestServicesDroppedByProfilesNeverDropsBaseServices(t *testing.T) {
	cf := testComposeFileWithProfiles()
	dropped := servicesDroppedByProfiles(cf, []string{"debug"}, nil)

	for _, name := range dropped {
		if name == "web" {
			t.Error("web has no profiles and should never be reported as dropped")
		}
	}
}
