package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const atmosAutostartFilename = "AtmosAgent.desktop"

type guiAutostartStatus struct {
	SchemaVersion  int    `json:"schemaVersion"`
	Enabled        bool   `json:"enabled"`
	OverrideHidden bool   `json:"overrideHidden"`
	ServiceEnabled bool   `json:"serviceEnabled"`
	OverridePath   string `json:"overridePath"`
	Service        string `json:"service"`
}

func handleGuiAutostart(command string, jsonOutput bool) error {
	switch command {
	case "status":
		status, err := guiAutostartStatusValue()
		if err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(status)
		}
		fmt.Println(formatGuiAutostartStatusText(status))
		return nil
	case "enable":
		if err := removeGuiAutostartOverride(); err != nil {
			return err
		}
		if err := runSystemctlUser("disable", atmosUserService); err != nil {
			return fmt.Errorf("gui-autostart partially enabled: removed hidden desktop override at %s, but disabling %s failed: %w", guiAutostartOverridePath(), atmosUserService, err)
		}
		return printCommandResult(jsonOutput, "gui-autostart.enable", "enabled")
	case "disable":
		if err := writeGuiAutostartSuppressOverride(); err != nil {
			return err
		}
		if err := runSystemctlUser("enable", atmosUserService); err != nil {
			return fmt.Errorf("gui-autostart partially disabled: wrote hidden desktop override at %s, but enabling %s failed: %w", guiAutostartOverridePath(), atmosUserService, err)
		}
		return printCommandResult(jsonOutput, "gui-autostart.disable", "disabled")
	default:
		return fmt.Errorf("unknown gui-autostart command %q", command)
	}
}

func guiAutostartStatusValue() (guiAutostartStatus, error) {
	overrideHidden, err := guiAutostartOverrideHidden()
	if err != nil {
		return guiAutostartStatus{}, err
	}

	serviceEnabled, err := userServiceEnabled()
	if err != nil {
		return guiAutostartStatus{}, err
	}

	return buildGuiAutostartStatus(overrideHidden, serviceEnabled), nil
}

func buildGuiAutostartStatus(overrideHidden, serviceEnabled bool) guiAutostartStatus {
	return guiAutostartStatus{
		SchemaVersion:  schemaVersion,
		Enabled:        !overrideHidden,
		OverrideHidden: overrideHidden,
		ServiceEnabled: serviceEnabled,
		OverridePath:   guiAutostartOverridePath(),
		Service:        atmosUserService,
	}
}

func formatGuiAutostartStatusText(status guiAutostartStatus) string {
	enabled, detail := formatGuiAutostartStatus(status.OverrideHidden, status.ServiceEnabled)
	if enabled {
		return "enabled\t" + detail
	}
	return "disabled\t" + detail
}

func formatGuiAutostartStatus(overrideHidden, serviceEnabled bool) (bool, string) {
	details := []string{}
	if overrideHidden {
		details = append(details, "GUI login launch suppressed")
	} else {
		details = append(details, "GUI login launch enabled")
	}
	if serviceEnabled {
		details = append(details, "user service enabled")
	} else {
		details = append(details, "user service disabled")
	}

	return !overrideHidden, strings.Join(details, ", ")
}

func guiAutostartOverrideHidden() (bool, error) {
	data, err := os.ReadFile(guiAutostartOverridePath())
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

func writeGuiAutostartSuppressOverride() error {
	path := guiAutostartOverridePath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	content := `[Desktop Entry]
Type=Application
Name=Atmos Agent
Comment=Suppresses the Atmos GUI at login; the user service starts the backend.
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

func removeGuiAutostartOverride() error {
	err := os.Remove(guiAutostartOverridePath())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func guiAutostartOverridePath() string {
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
