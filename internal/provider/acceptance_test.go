package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"github.com/ritajhq/terraform-provider-dockercompose/internal/docker"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// ============================================================
// Terraform Acceptance Tests
//
// These tests require Docker to be running and will create real containers.
// Run with: TF_ACC=1 go test -v -run TestAcc -timeout 10m
// ============================================================

var testAccProviders map[string]*schema.Provider
var testAccProvider *schema.Provider

func init() {
	testAccProvider = Provider()
	testAccProviders = map[string]*schema.Provider{
		"dockercompose": testAccProvider,
	}
}

func testAccPreCheck(t *testing.T) {
	// Check Docker is available
	client := &docker.DockerClient{Binary: "docker"}
	_, err := client.Version()
	if err != nil {
		t.Skipf("Docker not available, skipping acceptance test: %s", err)
	}
}

// ============================================================
// Stack resource acceptance tests
// ============================================================

func TestAccStackBasic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckStackDestroy("acc-basic"),
		Steps: []resource.TestStep{
			{
				Config: testAccStackConfigBasic(),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("dockercompose_stack.test", "name", "acc-basic"),
					resource.TestCheckResourceAttrSet("dockercompose_stack.test", "compose_yaml"),
					resource.TestCheckResourceAttrSet("dockercompose_stack.test", "compose_file_path"),
					testAccCheckStackRunning("acc-basic"),
					testAccCheckComposeFileExists("dockercompose_stack.test"),
					testAccCheckYAMLContains("dockercompose_stack.test", "image: nginx:alpine"),
					// Container runtime attributes
					resource.TestCheckResourceAttr("dockercompose_stack.test", "container.#", "1"),
					resource.TestCheckResourceAttr("dockercompose_stack.test", "container.0.service", "web"),
					resource.TestCheckResourceAttr("dockercompose_stack.test", "container.0.image", "nginx:alpine"),
					resource.TestCheckResourceAttr("dockercompose_stack.test", "container.0.state", "running"),
					resource.TestCheckResourceAttrSet("dockercompose_stack.test", "container.0.container_id"),
					resource.TestCheckResourceAttrSet("dockercompose_stack.test", "container.0.container_name"),
					resource.TestCheckResourceAttrSet("dockercompose_stack.test", "container.0.ip_address"),
				),
			},
		},
	})
}

func TestAccStackMultipleServices(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckStackDestroy("acc-multi"),
		Steps: []resource.TestStep{
			{
				Config: testAccStackConfigMultiService(),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("dockercompose_stack.multi", "name", "acc-multi"),
					testAccCheckStackRunning("acc-multi"),
					testAccCheckYAMLContains("dockercompose_stack.multi", "nginx:alpine"),
					testAccCheckYAMLContains("dockercompose_stack.multi", "redis:7-alpine"),
					// 2 containers sorted alphabetically by service: cache, web
					resource.TestCheckResourceAttr("dockercompose_stack.multi", "container.#", "2"),
					resource.TestCheckResourceAttr("dockercompose_stack.multi", "container.0.service", "cache"),
					resource.TestCheckResourceAttr("dockercompose_stack.multi", "container.0.image", "redis:7-alpine"),
					resource.TestCheckResourceAttr("dockercompose_stack.multi", "container.1.service", "web"),
					resource.TestCheckResourceAttr("dockercompose_stack.multi", "container.1.image", "nginx:alpine"),
					resource.TestCheckResourceAttrSet("dockercompose_stack.multi", "container.0.ip_address"),
					resource.TestCheckResourceAttrSet("dockercompose_stack.multi", "container.1.ip_address"),
				),
			},
		},
	})
}

func TestAccStackWithNetwork(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckStackDestroy("acc-net"),
		Steps: []resource.TestStep{
			{
				Config: testAccStackConfigWithNetwork(),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckStackRunning("acc-net"),
					testAccCheckYAMLContains("dockercompose_stack.nettest", "networks:"),
					// Container should have network_settings with the custom network
					resource.TestCheckResourceAttr("dockercompose_stack.nettest", "container.#", "1"),
					resource.TestCheckResourceAttr("dockercompose_stack.nettest", "container.0.network_settings.#", "1"),
					resource.TestCheckResourceAttrSet("dockercompose_stack.nettest", "container.0.network_settings.0.ip_address"),
				),
			},
		},
	})
}

// TestAccNetworkSharedAcrossStacks verifies the motivating use case for
// dockercompose_network: two independent stacks both joining the same
// literal Docker network (via external + external_name), rather than each
// getting its own project-prefixed network of the same short name.
func TestAccNetworkSharedAcrossStacks(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		CheckDestroy: resource.ComposeTestCheckFunc(
			testAccCheckStackDestroy("acc-shared-a"),
			testAccCheckStackDestroy("acc-shared-b"),
			testAccCheckNetworkDestroy("acc_shared_net"),
		),
		Steps: []resource.TestStep{
			{
				Config: testAccConfigSharedNetwork(),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("dockercompose_network.shared", "name", "acc_shared_net"),
					testAccCheckNetworkExists("acc_shared_net"),
					testAccCheckStackRunning("acc-shared-a"),
					testAccCheckStackRunning("acc-shared-b"),
					// Both stacks' generated YAML pin the same literal network name,
					// instead of letting Compose derive a project-prefixed one.
					testAccCheckYAMLContains("dockercompose_stack.a", "name: acc_shared_net"),
					testAccCheckYAMLContains("dockercompose_stack.b", "name: acc_shared_net"),
					// The network itself should show endpoints from both stacks' containers.
					testAccCheckNetworkContainerCount("acc_shared_net", 2),
				),
			},
		},
	})
}

func TestAccStackWithVolume(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckStackDestroy("acc-vol"),
		Steps: []resource.TestStep{
			{
				Config: testAccStackConfigWithVolume(),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckStackRunning("acc-vol"),
					testAccCheckYAMLContains("dockercompose_stack.voltest", "volumes:"),
				),
			},
		},
	})
}

func TestAccStackUpdate(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckStackDestroy("acc-update"),
		Steps: []resource.TestStep{
			{
				Config: testAccStackConfigUpdate("nginx:alpine"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckStackRunning("acc-update"),
					testAccCheckYAMLContains("dockercompose_stack.updatetest", "nginx:alpine"),
				),
			},
			{
				Config: testAccStackConfigUpdate("nginx:latest"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckStackRunning("acc-update"),
					testAccCheckYAMLContains("dockercompose_stack.updatetest", "nginx:latest"),
				),
			},
		},
	})
}

func TestAccStackWithEnvironment(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckStackDestroy("acc-env"),
		Steps: []resource.TestStep{
			{
				Config: testAccStackConfigWithEnv(),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckStackRunning("acc-env"),
					testAccCheckYAMLContains("dockercompose_stack.envtest", "APP_ENV: production"),
				),
			},
		},
	})
}

func TestAccStackWithHealthcheck(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckStackDestroy("acc-health"),
		Steps: []resource.TestStep{
			{
				Config: testAccStackConfigWithHealthcheck(),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckStackRunning("acc-health"),
					testAccCheckYAMLContains("dockercompose_stack.healthtest", "healthcheck:"),
					testAccCheckYAMLContains("dockercompose_stack.healthtest", "interval: 10s"),
				),
			},
		},
	})
}

func TestAccStackWithAllServiceOptions(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckStackDestroy("acc-full"),
		Steps: []resource.TestStep{
			{
				Config: testAccStackConfigFull(),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckStackRunning("acc-full"),
					testAccCheckYAMLContains("dockercompose_stack.fulltest", "hostname: webhost"),
					testAccCheckYAMLContains("dockercompose_stack.fulltest", "stop_signal: SIGTERM"),
					testAccCheckYAMLContains("dockercompose_stack.fulltest", "shm_size: 64m"),
				),
			},
		},
	})
}

func TestAccStackWithWatch(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckWatchStopped("dockercompose_stack.watchtest"),
		Steps: []resource.TestStep{
			{
				Config: testAccStackConfigWithWatch(),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckStackRunning("acc-watch"),
					testAccCheckYAMLContains("dockercompose_stack.watchtest", "develop:"),
					testAccCheckYAMLContains("dockercompose_stack.watchtest", "watch:"),
					resource.TestCheckResourceAttrSet("dockercompose_stack.watchtest", "watch_pid"),
					testAccCheckWatchPIDAlive("dockercompose_stack.watchtest"),
				),
			},
		},
	})
}

func TestAccStackWatchInheritsProviderDefault(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckWatchStopped("dockercompose_stack.inherittest"),
		Steps: []resource.TestStep{
			{
				// Provider sets watch = true; the resource doesn't set watch at all,
				// so it must inherit true and launch the watcher.
				Config: testAccProviderConfig(true, nil) + `
resource "dockercompose_stack" "inherittest" {
  name = "acc-watch-inherit"

  service {
    name  = "web"
    image = "nginx:alpine"
    ports = ["18095:80"]

    develop_watch {
      path   = "."
      action = "sync"
      target = "/usr/share/nginx/html"
    }
  }
}
`,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckStackRunning("acc-watch-inherit"),
					resource.TestCheckResourceAttr("dockercompose_stack.inherittest", "watch", "true"),
					testAccCheckWatchPIDAlive("dockercompose_stack.inherittest"),
				),
			},
		},
	})
}

func TestAccStackWatchResourceOverridesProviderDefault(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckWatchStopped("dockercompose_stack.overridetest"),
		Steps: []resource.TestStep{
			{
				// Provider default is watch = true, but this resource explicitly sets
				// watch = false, which must win and prevent the watcher from starting.
				Config: testAccProviderConfig(true, nil) + `
resource "dockercompose_stack" "overridetest" {
  name  = "acc-watch-override"
  watch = false

  service {
    name  = "web"
    image = "nginx:alpine"
    ports = ["18096:80"]

    develop_watch {
      path   = "."
      action = "sync"
      target = "/usr/share/nginx/html"
    }
  }
}
`,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckStackRunning("acc-watch-override"),
					resource.TestCheckResourceAttr("dockercompose_stack.overridetest", "watch", "false"),
					resource.TestCheckResourceAttr("dockercompose_stack.overridetest", "watch_pid", "0"),
				),
			},
		},
	})
}

// ============================================================
// active_profiles acceptance tests
// ============================================================

func TestAccStackActiveProfilesResourceLevel(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckStackDestroy("acc-profiles"),
		Steps: []resource.TestStep{
			{
				// "debug" profile active: both web and debug-tagged service should run.
				Config: testAccStackConfigWithProfiles([]string{"debug"}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckStackRunning("acc-profiles"),
					testAccCheckServiceContainerCount("acc-profiles", "debugger", 1),
				),
			},
			{
				// Profile removed: the debug service must be stopped by Update.
				Config: testAccStackConfigWithProfiles(nil),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckStackRunning("acc-profiles"),
					testAccCheckServiceContainerCount("acc-profiles", "debugger", 0),
				),
			},
		},
	})
}

func TestAccStackActiveProfilesInheritsProviderDefault(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckStackDestroy("acc-profiles-inherit"),
		Steps: []resource.TestStep{
			{
				// Provider activates "debug" by default; resource doesn't set active_profiles.
				Config: testAccProviderConfig(false, []string{"debug"}) + `
resource "dockercompose_stack" "profilesinherit" {
  name = "acc-profiles-inherit"

  service {
    name  = "web"
    image = "nginx:alpine"
    ports = ["18097:80"]
  }

  service {
    name     = "debugger"
    image    = "alpine:latest"
    command  = ["sleep", "3600"]
    profiles = ["debug"]
  }
}
`,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckStackRunning("acc-profiles-inherit"),
					resource.TestCheckResourceAttr("dockercompose_stack.profilesinherit", "active_profiles.#", "1"),
					resource.TestCheckResourceAttr("dockercompose_stack.profilesinherit", "active_profiles.0", "debug"),
					testAccCheckServiceContainerCount("acc-profiles-inherit", "debugger", 1),
				),
			},
		},
	})
}

func TestAccStackActiveProfilesResourceOverridesProviderDefault(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckStackDestroy("acc-profiles-override"),
		Steps: []resource.TestStep{
			{
				// Provider activates "debug" by default, but this resource explicitly
				// overrides active_profiles to [] — the debug service must not start.
				Config: testAccProviderConfig(false, []string{"debug"}) + `
resource "dockercompose_stack" "profilesoverride" {
  name            = "acc-profiles-override"
  active_profiles = []

  service {
    name  = "web"
    image = "nginx:alpine"
    ports = ["18098:80"]
  }

  service {
    name     = "debugger"
    image    = "alpine:latest"
    command  = ["sleep", "3600"]
    profiles = ["debug"]
  }
}
`,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckStackRunning("acc-profiles-override"),
					resource.TestCheckResourceAttr("dockercompose_stack.profilesoverride", "active_profiles.#", "0"),
					testAccCheckServiceContainerCount("acc-profiles-override", "debugger", 0),
				),
			},
		},
	})
}

// ============================================================
// Project resource acceptance tests
// ============================================================

func TestAccProjectInlineYAML(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckStackDestroy("acc-project-inline"),
		Steps: []resource.TestStep{
			{
				Config: testAccProjectConfigInline(),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("dockercompose_project.inline", "name", "acc-project-inline"),
					resource.TestCheckResourceAttrSet("dockercompose_project.inline", "yaml_sha256"),
					testAccCheckStackRunning("acc-project-inline"),
					// Container attributes also work on dockercompose_project
					resource.TestCheckResourceAttr("dockercompose_project.inline", "container.#", "1"),
					resource.TestCheckResourceAttr("dockercompose_project.inline", "container.0.service", "web"),
					resource.TestCheckResourceAttr("dockercompose_project.inline", "container.0.state", "running"),
					resource.TestCheckResourceAttrSet("dockercompose_project.inline", "container.0.container_id"),
					resource.TestCheckResourceAttrSet("dockercompose_project.inline", "container.0.ip_address"),
				),
			},
		},
	})
}

func TestAccProjectFromFile(t *testing.T) {
	// Create a temporary compose file
	tmpDir := t.TempDir()
	composeFile := filepath.Join(tmpDir, "docker-compose.yml")
	content := `services:
  web:
    image: nginx:alpine
    ports:
      - "18082:80"
`
	if err := os.WriteFile(composeFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create temp compose file: %s", err)
	}

	// Convert backslashes to forward slashes for Terraform HCL
	hclPath := strings.ReplaceAll(composeFile, "\\", "/")

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckStackDestroy("acc-project-file"),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "dockercompose_project" "fromfile" {
  name         = "acc-project-file"
  compose_file = "%s"
}
`, hclPath),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckStackRunning("acc-project-file"),
				),
			},
		},
	})
}

// ============================================================
// Regression tests
// ============================================================

func TestAccStackDestroyRemovesContainers(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckStackDestroy("acc-destroy-test"),
		Steps: []resource.TestStep{
			{
				Config: `
resource "dockercompose_stack" "destroytest" {
  name = "acc-destroy-test"
  service {
    name  = "web"
    image = "nginx:alpine"
  }
}
`,
				Check: testAccCheckStackRunning("acc-destroy-test"),
			},
		},
	})
}

func TestAccStackEmptyPortsList(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckStackDestroy("acc-no-ports"),
		Steps: []resource.TestStep{
			{
				Config: `
resource "dockercompose_stack" "noports" {
  name = "acc-no-ports"
  service {
    name  = "worker"
    image = "alpine:latest"
    command = ["sleep", "3600"]
  }
}
`,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckStackRunning("acc-no-ports"),
					testAccCheckYAMLNotContains("dockercompose_stack.noports", "ports:"),
				),
			},
		},
	})
}

func TestAccStackIdempotent(t *testing.T) {
	config := `
resource "dockercompose_stack" "idempotent" {
  name = "acc-idempotent"
  service {
    name  = "web"
    image = "nginx:alpine"
    ports = ["18083:80"]
  }
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckStackDestroy("acc-idempotent"),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check:  testAccCheckStackRunning("acc-idempotent"),
			},
			{
				// Apply same config again — should produce no changes
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

// ============================================================
// Check functions (test helpers)
// ============================================================

func testAccCheckStackRunning(projectName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client := &docker.DockerClient{Binary: "docker"}
		output, err := client.ComposePSServices(projectName, "")
		if err != nil {
			return fmt.Errorf("docker compose ps failed for project %q: %s", projectName, err)
		}
		if strings.TrimSpace(output) == "" {
			return fmt.Errorf("no running services found for project %q", projectName)
		}
		return nil
	}
}

func testAccCheckStackDestroy(projectName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client := &docker.DockerClient{Binary: "docker"}
		output, _ := client.ComposePSServices(projectName, "")
		if strings.TrimSpace(output) != "" {
			return fmt.Errorf("stack %q still has running services: %s", projectName, output)
		}
		return nil
	}
}

func testAccCheckNetworkExists(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client := &docker.DockerClient{Binary: "docker"}
		if _, err := client.NetworkInspect(name); err != nil {
			return fmt.Errorf("network %q does not exist: %s", name, err)
		}
		return nil
	}
}

func testAccCheckNetworkDestroy(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client := &docker.DockerClient{Binary: "docker"}
		if _, err := client.NetworkInspect(name); err == nil {
			return fmt.Errorf("network %q still exists after destroy", name)
		}
		return nil
	}
}

// testAccCheckNetworkContainerCount verifies how many containers currently have an
// endpoint on the given network, used to confirm multiple stacks actually joined
// the same shared network rather than each getting its own.
func testAccCheckNetworkContainerCount(name string, want int) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client := &docker.DockerClient{Binary: "docker"}
		out, err := client.NetworkInspect(name)
		if err != nil {
			return fmt.Errorf("network %q inspect failed: %s", name, err)
		}

		var inspected []struct {
			Containers map[string]interface{} `json:"Containers"`
		}
		if err := json.Unmarshal([]byte(out), &inspected); err != nil {
			return fmt.Errorf("error parsing network inspect output for %q: %s", name, err)
		}
		if len(inspected) == 0 {
			return fmt.Errorf("network %q not found in inspect output", name)
		}

		got := len(inspected[0].Containers)
		if got != want {
			return fmt.Errorf("network %q: got %d attached containers, want %d", name, got, want)
		}
		return nil
	}
}

// testAccCheckWatchPIDAlive verifies that watch_pid in state points at a live process.
func testAccCheckWatchPIDAlive(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %q not found in state", resourceName)
		}

		pidStr := rs.Primary.Attributes["watch_pid"]
		pid, err := strconv.Atoi(pidStr)
		if err != nil || pid == 0 {
			return fmt.Errorf("watch_pid is not a live PID: %q", pidStr)
		}
		if !docker.IsProcessAlive(pid) {
			return fmt.Errorf("watch_pid %d is not alive", pid)
		}
		return nil
	}
}

// testAccCheckWatchStopped is a CheckDestroy function verifying that the watcher
// process recorded for resourceName is no longer running after destroy.
func testAccCheckWatchStopped(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "dockercompose_stack" && rs.Type != "dockercompose_project" {
				continue
			}
			pidStr := rs.Primary.Attributes["watch_pid"]
			if pidStr == "" || pidStr == "0" {
				continue
			}
			pid, err := strconv.Atoi(pidStr)
			if err != nil {
				continue
			}
			if docker.IsProcessAlive(pid) {
				return fmt.Errorf("watcher pid %d for %q is still alive after destroy", pid, resourceName)
			}
		}
		return nil
	}
}

// testAccCheckServiceContainerCount verifies how many *running* containers exist for
// a given service within projectName, used to confirm a profile-scoped service is
// running (count 1) or has been stopped by an active_profiles change (count 0).
// It queries `ps -a` (unscoped by profile, listing all containers regardless of
// profile) and inspects each entry's actual State, since a --profile-scoped `ps`
// only reflects service *definitions* in that profile set, not whether a container
// left over from a previous profile is still running.
func testAccCheckServiceContainerCount(projectName, serviceName string, want int) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client := &docker.DockerClient{Binary: "docker"}
		psJSON, err := client.ComposePSJSON(projectName, "")
		if err != nil {
			if want == 0 {
				return nil
			}
			return fmt.Errorf("docker compose ps failed for project %q: %s", projectName, err)
		}

		entries, err := parseComposePSJSON(psJSON)
		if err != nil {
			return fmt.Errorf("error parsing docker compose ps output: %s", err)
		}

		count := 0
		for _, e := range entries {
			if e.Service == serviceName && e.State == "running" {
				count++
			}
		}

		if count != want {
			return fmt.Errorf("service %q in project %q: got %d running, want %d", serviceName, projectName, count, want)
		}
		return nil
	}
}

func testAccCheckComposeFileExists(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %q not found in state", resourceName)
		}

		filePath := rs.Primary.Attributes["compose_file_path"]
		if filePath == "" {
			return fmt.Errorf("compose_file_path is empty")
		}

		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			return fmt.Errorf("compose file does not exist: %s", filePath)
		}
		return nil
	}
}

func testAccCheckYAMLContains(resourceName, substr string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %q not found in state", resourceName)
		}

		yaml := rs.Primary.Attributes["compose_yaml"]
		if yaml == "" {
			return fmt.Errorf("compose_yaml is empty")
		}
		if !strings.Contains(yaml, substr) {
			return fmt.Errorf("compose_yaml does not contain %q.\nFull YAML:\n%s", substr, yaml)
		}
		return nil
	}
}

func testAccCheckYAMLNotContains(resourceName, substr string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %q not found in state", resourceName)
		}

		yaml := rs.Primary.Attributes["compose_yaml"]
		if strings.Contains(yaml, substr) {
			return fmt.Errorf("compose_yaml should NOT contain %q.\nFull YAML:\n%s", substr, yaml)
		}
		return nil
	}
}

// ============================================================
// Test configs
// ============================================================

// testAccProviderConfig renders a `provider "dockercompose" {}` block setting the
// provider-level watch and active_profiles defaults under test.
func testAccProviderConfig(watch bool, activeProfiles []string) string {
	profilesLit := "[]"
	if len(activeProfiles) > 0 {
		quoted := make([]string, len(activeProfiles))
		for i, p := range activeProfiles {
			quoted[i] = fmt.Sprintf("%q", p)
		}
		profilesLit = "[" + strings.Join(quoted, ", ") + "]"
	}

	return fmt.Sprintf(`
provider "dockercompose" {
  watch           = %t
  active_profiles = %s
}
`, watch, profilesLit)
}

// testAccStackConfigWithProfiles renders a stack with a base "web" service and a
// "debugger" service tagged with the "debug" profile, setting active_profiles to
// the given resource-level value (nil renders an empty list).
func testAccStackConfigWithProfiles(activeProfiles []string) string {
	quoted := make([]string, len(activeProfiles))
	for i, p := range activeProfiles {
		quoted[i] = fmt.Sprintf("%q", p)
	}
	profilesLit := "[" + strings.Join(quoted, ", ") + "]"

	return fmt.Sprintf(`
resource "dockercompose_stack" "profilestest" {
  name            = "acc-profiles"
  active_profiles = %s

  service {
    name  = "web"
    image = "nginx:alpine"
    ports = ["18099:80"]
  }

  service {
    name     = "debugger"
    image    = "alpine:latest"
    command  = ["sleep", "3600"]
    profiles = ["debug"]
  }
}
`, profilesLit)
}

func testAccStackConfigBasic() string {
	return `
resource "dockercompose_stack" "test" {
  name = "acc-basic"
  service {
    name  = "web"
    image = "nginx:alpine"
    ports = ["18080:80"]
  }
}
`
}

func testAccStackConfigMultiService() string {
	return `
resource "dockercompose_stack" "multi" {
  name = "acc-multi"

  service {
    name  = "web"
    image = "nginx:alpine"
    ports = ["18090:80"]
  }

  service {
    name  = "cache"
    image = "redis:7-alpine"
  }
}
`
}

func testAccStackConfigWithNetwork() string {
	return `
resource "dockercompose_stack" "nettest" {
  name = "acc-net"

  service {
    name     = "web"
    image    = "nginx:alpine"
    networks = ["mynet"]
  }

  network {
    name   = "mynet"
    driver = "bridge"
  }
}
`
}

func testAccConfigSharedNetwork() string {
	return `
resource "dockercompose_network" "shared" {
  name       = "acc_shared_net"
  driver     = "bridge"
  attachable = true
}

resource "dockercompose_stack" "a" {
  name = "acc-shared-a"

  service {
    name     = "web"
    image    = "nginx:alpine"
    networks = ["acc_shared_net"]
  }

  network {
    name          = "acc_shared_net"
    external      = true
    external_name = dockercompose_network.shared.name
  }

  depends_on = [dockercompose_network.shared]
}

resource "dockercompose_stack" "b" {
  name = "acc-shared-b"

  service {
    name     = "web"
    image    = "nginx:alpine"
    networks = ["acc_shared_net"]
  }

  network {
    name          = "acc_shared_net"
    external      = true
    external_name = dockercompose_network.shared.name
  }

  depends_on = [dockercompose_network.shared]
}
`
}

func testAccStackConfigWithVolume() string {
	return `
resource "dockercompose_stack" "voltest" {
  name = "acc-vol"

  service {
    name    = "web"
    image   = "nginx:alpine"
    volumes = ["testdata:/data"]
  }

  volume {
    name = "testdata"
  }
}
`
}

func testAccStackConfigUpdate(image string) string {
	return fmt.Sprintf(`
resource "dockercompose_stack" "updatetest" {
  name = "acc-update"
  service {
    name  = "web"
    image = "%s"
    ports = ["18091:80"]
  }
}
`, image)
}

func testAccStackConfigWithEnv() string {
	return `
resource "dockercompose_stack" "envtest" {
  name = "acc-env"
  service {
    name  = "web"
    image = "nginx:alpine"
    environment = {
      APP_ENV = "production"
      DEBUG   = "false"
    }
  }
}
`
}

func testAccStackConfigWithHealthcheck() string {
	return `
resource "dockercompose_stack" "healthtest" {
  name = "acc-health"
  service {
    name  = "web"
    image = "nginx:alpine"
    ports = ["18092:80"]

    healthcheck_test     = ["CMD", "curl", "-f", "http://localhost"]
    healthcheck_interval = "10s"
    healthcheck_timeout  = "5s"
    healthcheck_retries  = 3
  }
}
`
}

func testAccStackConfigWithWatch() string {
	return `
resource "dockercompose_stack" "watchtest" {
  name  = "acc-watch"
  watch = true

  service {
    name  = "web"
    image = "nginx:alpine"
    ports = ["18094:80"]

    develop_watch {
      path   = "."
      action = "sync"
      target = "/usr/share/nginx/html"
      ignore = [".git/"]
    }
  }
}
`
}

func testAccStackConfigFull() string {
	return `
resource "dockercompose_stack" "fulltest" {
  name = "acc-full"

  service {
    name       = "web"
    image      = "nginx:alpine"
    restart    = "unless-stopped"
    ports      = ["18093:80"]
    hostname   = "webhost"
    shm_size   = "64m"
    stop_signal = "SIGTERM"
    stop_grace_period = "10s"

    labels = {
      "test.label" = "value"
    }

    environment = {
      TEST_VAR = "hello"
    }
  }
}
`
}

func testAccProjectConfigInline() string {
	return `
resource "dockercompose_project" "inline" {
  name = "acc-project-inline"
  compose_yaml = <<-EOT
    services:
      web:
        image: nginx:alpine
        ports:
          - "18081:80"
  EOT
}
`
}
