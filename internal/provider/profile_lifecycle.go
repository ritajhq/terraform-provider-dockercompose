package provider

import (
	"fmt"

	"github.com/xRizur/terraform-provider-dockercompose/internal/docker"
)

// servicesActiveForProfiles returns the names of services in cf that would be started
// under the given active profile set: services with no profiles are always active;
// services with profiles are active only if at least one of their profiles is active.
func servicesActiveForProfiles(cf *docker.ComposeFile, activeProfiles []string) map[string]bool {
	active := make(map[string]bool, len(activeProfiles))
	for _, p := range activeProfiles {
		active[p] = true
	}

	result := make(map[string]bool, len(cf.Services))
	for name, svc := range cf.Services {
		if len(svc.Profiles) == 0 {
			result[name] = true
			continue
		}
		for _, p := range svc.Profiles {
			if active[p] {
				result[name] = true
				break
			}
		}
	}
	return result
}

// servicesDroppedByProfiles returns the services that were active under oldProfiles
// but are no longer active under newProfiles, so they can be stopped (not removed)
// before re-applying the compose file with the new profile set.
func servicesDroppedByProfiles(cf *docker.ComposeFile, oldProfiles, newProfiles []string) []string {
	wasActive := servicesActiveForProfiles(cf, oldProfiles)
	isActive := servicesActiveForProfiles(cf, newProfiles)

	dropped := make([]string, 0)
	for name := range wasActive {
		if wasActive[name] && !isActive[name] {
			dropped = append(dropped, name)
		}
	}
	return dropped
}

// stopDroppedProfileServices stops (without removing) any services that were active
// under the previously-applied profile set but have dropped out of the newly resolved
// one, so they don't keep running once their profile is no longer active.
func stopDroppedProfileServices(client *docker.DockerClient, stackName, composeFilePath string, cf *docker.ComposeFile, oldProfiles, newProfiles []string) error {
	dropped := servicesDroppedByProfiles(cf, oldProfiles, newProfiles)
	if len(dropped) == 0 {
		return nil
	}

	if _, err := client.ComposeStop(stackName, composeFilePath, dropped); err != nil {
		return fmt.Errorf("error stopping services dropped from active_profiles: %s", err)
	}
	return nil
}
