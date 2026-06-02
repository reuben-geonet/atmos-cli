package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	atmosUserService     = "atmos-agent.service"
	atmosUserServiceUnit = "/usr/lib/systemd/user/atmos-agent.service"
)

type serviceActivity struct {
	active bool
	state  string
}

func userServiceEnabled() (bool, error) {
	cmd := exec.Command("systemctl", "--user", "is-enabled", atmosUserService)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return strings.TrimSpace(string(output)) == "enabled", nil
	}

	text := strings.TrimSpace(string(output))
	if strings.Contains(text, "disabled") {
		return false, nil
	}

	return false, fmt.Errorf("systemctl --user is-enabled %s failed: %w: %s", atmosUserService, err, text)
}

func userServiceActive() (serviceActivity, error) {
	cmd := exec.Command("systemctl", "--user", "is-active", atmosUserService)
	output, err := cmd.CombinedOutput()
	return parseServiceActiveOutput(string(output), err)
}

func parseServiceActiveOutput(output string, err error) (serviceActivity, error) {
	state := strings.TrimSpace(output)
	if state == "" {
		state = "unknown"
	}

	switch state {
	case "active":
		return serviceActivity{active: true, state: state}, nil
	case "inactive", "failed", "activating", "deactivating", "reloading", "unknown":
		return serviceActivity{active: false, state: state}, nil
	}

	if err != nil {
		return serviceActivity{active: false, state: state}, fmt.Errorf("systemctl --user is-active %s failed: %w: %s", atmosUserService, err, state)
	}

	return serviceActivity{active: false, state: state}, nil
}

func runSystemctlUser(args ...string) error {
	if _, err := os.Stat(atmosUserServiceUnit); err != nil {
		return fmt.Errorf("%s is not installed: %w", atmosUserServiceUnit, err)
	}

	cmd := exec.Command("systemctl", append([]string{"--user"}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl --user %s failed: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}
