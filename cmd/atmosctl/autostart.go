package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const atmosAutostartFilename = "AtmosAgent.desktop"

type autostartStatus struct {
	SchemaVersion  int    `json:"schemaVersion"`
	Enabled        bool   `json:"enabled"`
	OverrideHidden bool   `json:"overrideHidden"`
	ServiceEnabled bool   `json:"serviceEnabled"`
	OverridePath   string `json:"overridePath"`
	Service        string `json:"service"`
}

func handleAutostart(command string, jsonOutput bool) error {
	switch command {
	case "status":
		status, err := quietAutostartStatus()
		if err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(status)
		}
		fmt.Println(formatAutostartStatusText(status))
		return nil
	case "enable":
		if err := writeQuietAutostartOverride(); err != nil {
			return err
		}
		if err := runSystemctlUser("enable", atmosUserService); err != nil {
			return fmt.Errorf("autostart partially configured: wrote hidden desktop override at %s, but enabling %s failed: %w", quietAutostartPath(), atmosUserService, err)
		}
		return printCommandResult(jsonOutput, "autostart.enable", "enabled")
	case "disable":
		if err := removeQuietAutostartOverride(); err != nil {
			return err
		}
		if err := runSystemctlUser("disable", atmosUserService); err != nil {
			return fmt.Errorf("autostart partially disabled: removed desktop override at %s, but disabling %s failed: %w", quietAutostartPath(), atmosUserService, err)
		}
		return printCommandResult(jsonOutput, "autostart.disable", "disabled")
	default:
		return fmt.Errorf("unknown autostart command %q", command)
	}
}

func quietAutostartStatus() (autostartStatus, error) {
	overrideHidden, err := quietAutostartOverrideHidden()
	if err != nil {
		return autostartStatus{}, err
	}

	serviceEnabled, err := userServiceEnabled()
	if err != nil {
		return autostartStatus{}, err
	}

	return buildAutostartStatus(overrideHidden, serviceEnabled), nil
}

func buildAutostartStatus(overrideHidden, serviceEnabled bool) autostartStatus {
	return autostartStatus{
		SchemaVersion:  schemaVersion,
		Enabled:        overrideHidden && serviceEnabled,
		OverrideHidden: overrideHidden,
		ServiceEnabled: serviceEnabled,
		OverridePath:   quietAutostartPath(),
		Service:        atmosUserService,
	}
}

func formatAutostartStatusText(status autostartStatus) string {
	enabled, detail := formatQuietAutostartStatus(status.OverrideHidden, status.ServiceEnabled)
	if enabled {
		return "enabled\t" + detail
	}
	return "disabled\t" + detail
}

func formatQuietAutostartStatus(overrideHidden, serviceEnabled bool) (bool, string) {
	details := []string{}
	if overrideHidden {
		details = append(details, "autostart override hidden")
	} else {
		details = append(details, "autostart override absent")
	}
	if serviceEnabled {
		details = append(details, "user service enabled")
	} else {
		details = append(details, "user service disabled")
	}

	return overrideHidden && serviceEnabled, strings.Join(details, ", ")
}

func quietAutostartOverrideHidden() (bool, error) {
	data, err := os.ReadFile(quietAutostartPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}

	return desktopEntryHidden(string(data)), nil
}

func desktopEntryHidden(data string) bool {
	for _, line := range strings.Split(data, "\n") {
		key, value, ok := strings.Cut(line, "=")
		if ok && strings.EqualFold(strings.TrimSpace(key), "Hidden") {
			return strings.EqualFold(strings.TrimSpace(value), "true")
		}
	}
	return false
}

func writeQuietAutostartOverride() error {
	path := quietAutostartPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	content := `[Desktop Entry]
Type=Application
Name=Atmos Agent
Comment=Disabled by Atmos GNOME Shell integration; the user service starts the backend.
Exec=/usr/bin/atmos
Terminal=false
Hidden=true
`
	tmp, err := os.CreateTemp(dir, atmosAutostartFilename+".")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmpPath, path)
}

func removeQuietAutostartOverride() error {
	err := os.Remove(quietAutostartPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func quietAutostartPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil || configDir == "" {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil || home == "" {
			return filepath.Join(".config", "autostart", atmosAutostartFilename)
		}
		configDir = filepath.Join(home, ".config")
	}
	return filepath.Join(configDir, "autostart", atmosAutostartFilename)
}
