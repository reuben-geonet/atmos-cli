package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	version = "0.1.0"

	schemaVersion = 1

	defaultBackendAddr = "127.0.0.1:6668"

	atmosInterfaceName     = "atmos"
	atmosAutostartFilename = "AtmosAgent.desktop"
	atmosUserService       = "atmos-agent.service"
	atmosUserServiceUnit   = "/usr/lib/systemd/user/atmos-agent.service"

	subjectStart = "tunnel.Start"
	subjectStop  = "tunnel.Stop"

	reasonInterfaceMissing = "interface_missing"
	reasonInterfaceDown    = "interface_down"
	reasonNoAtmosAddress   = "no_atmos_address"
)

type message struct {
	Subject         string          `json:"subject,omitempty"`
	ReplyID         string          `json:"replyID,omitempty"`
	RequestID       string          `json:"RequestID,omitempty"`
	PayloadRaw      json.RawMessage `json:"payloadRaw,omitempty"`
	PayloadIsStream bool            `json:"payloadIsStream"`
}

type versionOutput struct {
	SchemaVersion int    `json:"schemaVersion"`
	Version       string `json:"version"`
}

type commandResult struct {
	SchemaVersion int    `json:"schemaVersion"`
	OK            bool   `json:"ok"`
	Command       string `json:"command"`
}

type vpnStatus struct {
	SchemaVersion int      `json:"schemaVersion"`
	State         string   `json:"state"`
	Interface     string   `json:"interface"`
	Addresses     []string `json:"addresses"`
	Reason        string   `json:"reason,omitempty"`
	Service       string   `json:"service"`
	ServiceActive bool     `json:"serviceActive"`
	ServiceState  string   `json:"serviceState"`
}

type autostartStatus struct {
	SchemaVersion  int    `json:"schemaVersion"`
	Enabled        bool   `json:"enabled"`
	OverrideHidden bool   `json:"overrideHidden"`
	ServiceEnabled bool   `json:"serviceEnabled"`
	OverridePath   string `json:"overridePath"`
	Service        string `json:"service"`
}

type vpnCommand struct {
	subject     string
	canonical   string
	sendsPubsub bool
}

type serviceActivity struct {
	active bool
	state  string
}

func main() {
	addr := flag.String("addr", defaultBackendAddr, "Atmos backend pubsub address")
	timeout := flag.Duration("timeout", 2*time.Second, "TCP connect/write timeout")
	jsonOutput := flag.Bool("json", false, "print machine-readable JSON")
	flag.Parse()

	if flag.NArg() < 1 {
		usage()
		os.Exit(2)
	}

	var err error
	switch flag.Arg(0) {
	case "version":
		if flag.NArg() != 1 {
			usage()
			os.Exit(2)
		}
		if *jsonOutput {
			err = printJSON(versionOutput{
				SchemaVersion: schemaVersion,
				Version:       version,
			})
			break
		}
		fmt.Printf("atmosctl %s\n", version)
		return
	case "vpn":
		if flag.NArg() != 2 {
			usage()
			os.Exit(2)
		}
		err = handleVPN(flag.Arg(1), *addr, *timeout, *jsonOutput)
	case "autostart":
		if flag.NArg() != 2 {
			usage()
			os.Exit(2)
		}
		err = handleAutostart(flag.Arg(1), *jsonOutput)
	default:
		usage()
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: %s [--addr %s] [--timeout 2s] [--json] version | vpn status|pause|resume | autostart status|enable|disable\n", os.Args[0], defaultBackendAddr)
}

func handleVPN(command, addr string, timeout time.Duration, jsonOutput bool) error {
	info, err := vpnCommandInfo(command)
	if err != nil {
		return err
	}
	if !info.sendsPubsub {
		return printVPNStatus(jsonOutput)
	}
	if err := sendSubject(addr, info.subject, timeout); err != nil {
		return err
	}
	return printCommandResult(jsonOutput, info.canonical, "")
}

func vpnCommandSubject(command string) (string, bool, error) {
	info, err := vpnCommandInfo(command)
	return info.subject, info.sendsPubsub, err
}

func vpnCommandInfo(command string) (vpnCommand, error) {
	switch command {
	case "status":
		return vpnCommand{canonical: "vpn.status"}, nil
	case "pause", "stop":
		return vpnCommand{
			subject:     subjectStop,
			canonical:   "vpn.pause",
			sendsPubsub: true,
		}, nil
	case "resume", "start":
		return vpnCommand{
			subject:     subjectStart,
			canonical:   "vpn.resume",
			sendsPubsub: true,
		}, nil
	default:
		return vpnCommand{}, fmt.Errorf("unknown vpn command %q", command)
	}
}

func printVPNStatus(jsonOutput bool) error {
	status, err := atmosInterfaceStatus()
	if err != nil {
		return err
	}

	if jsonOutput {
		return printJSON(status)
	}

	fmt.Println(formatVPNStatusText(status))
	return nil
}

func formatVPNStatusText(status vpnStatus) string {
	details := vpnStatusDetails(status)
	if len(details) == 0 {
		return vpnStatusTextState(status)
	}

	return fmt.Sprintf("%s\t%s", vpnStatusTextState(status), strings.Join(details, ", "))
}

func vpnStatusTextState(status vpnStatus) string {
	if status.ServiceActive {
		return status.State
	}
	return "agent-" + status.ServiceState
}

func vpnStatusDetails(status vpnStatus) []string {
	var details []string
	details = append(details, "service "+status.ServiceState)
	switch status.Reason {
	case reasonInterfaceMissing:
		details = append(details, "interface missing")
	case reasonInterfaceDown:
		details = append(details, "interface down")
	}
	if status.Reason != reasonInterfaceDown && len(status.Addresses) > 0 {
		details = append(details, "addresses "+strings.Join(status.Addresses, ","))
	}
	return details
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

func atmosInterfaceStatus() (vpnStatus, error) {
	service, err := userServiceActive()
	if err != nil {
		return vpnStatus{}, err
	}

	iface, err := net.InterfaceByName(atmosInterfaceName)
	if err != nil {
		if errors.Is(err, net.ErrClosed) || strings.Contains(err.Error(), "no such network interface") {
			return newVPNStatus("disconnected", nil, reasonInterfaceMissing, service), nil
		}
		return vpnStatus{}, err
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return vpnStatus{}, err
	}

	var addressTexts []string
	for _, addr := range addrs {
		addressTexts = append(addressTexts, addr.String())
	}

	return classifyAtmosInterfaceStatus(iface.Flags, addressTexts, service), nil
}

func classifyAtmosInterfaceStatus(flags net.Flags, addresses []string, service serviceActivity) vpnStatus {
	if flags&net.FlagUp == 0 {
		return newVPNStatus("disconnected", addresses, reasonInterfaceDown, service)
	}

	for _, address := range addresses {
		if strings.HasPrefix(address, "100.65.") {
			return newVPNStatus("connected", addresses, "", service)
		}
	}

	return newVPNStatus("unknown", addresses, reasonNoAtmosAddress, service)
}

func newVPNStatus(state string, addresses []string, reason string, service serviceActivity) vpnStatus {
	if addresses == nil {
		addresses = []string{}
	}
	if service.state == "" {
		service.state = "unknown"
	}
	return vpnStatus{
		SchemaVersion: schemaVersion,
		State:         state,
		Interface:     atmosInterfaceName,
		Addresses:     addresses,
		Reason:        reason,
		Service:       atmosUserService,
		ServiceActive: service.active,
		ServiceState:  service.state,
	}
}

func sendSubject(addr, subject string, timeout time.Duration) error {
	frame, err := buildFrame(subject)
	if err != nil {
		return err
	}

	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return fmt.Errorf("Atmos backend is not reachable at %s; is %s running? %w", addr, atmosUserService, err)
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return err
	}

	_, err = conn.Write(frame)
	return err
}

func buildFrame(subject string) ([]byte, error) {
	msg := message{
		Subject:         subject,
		PayloadRaw:      json.RawMessage(`{}`),
		PayloadIsStream: false,
	}

	frame, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	return append(frame, 0), nil
}

func printCommandResult(jsonOutput bool, command, text string) error {
	if !jsonOutput {
		if text != "" {
			fmt.Println(text)
		}
		return nil
	}
	return printJSON(commandResult{
		SchemaVersion: schemaVersion,
		OK:            true,
		Command:       command,
	})
}

func printJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}
