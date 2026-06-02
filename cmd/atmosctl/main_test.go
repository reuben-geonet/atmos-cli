package main

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildFrame(t *testing.T) {
	frame, err := buildFrame(subjectStop)
	if err != nil {
		t.Fatal(err)
	}
	if len(frame) == 0 || frame[len(frame)-1] != 0 {
		t.Fatalf("frame does not end with NUL: %q", frame)
	}

	var msg message
	if err := json.Unmarshal(frame[:len(frame)-1], &msg); err != nil {
		t.Fatal(err)
	}

	if msg.Subject != subjectStop {
		t.Fatalf("subject = %q, want %q", msg.Subject, subjectStop)
	}
	if string(msg.PayloadRaw) != "{}" {
		t.Fatalf("payloadRaw = %s, want {}", msg.PayloadRaw)
	}
	if msg.PayloadIsStream {
		t.Fatal("payloadIsStream = true, want false")
	}
}

func TestVPNCommandSubject(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		wantSubject string
		wantSend    bool
		wantErr     bool
	}{
		{name: "status", command: "status"},
		{name: "pause", command: "pause", wantSubject: subjectStop, wantSend: true},
		{name: "stop alias", command: "stop", wantSubject: subjectStop, wantSend: true},
		{name: "resume", command: "resume", wantSubject: subjectStart, wantSend: true},
		{name: "start alias", command: "start", wantSubject: subjectStart, wantSend: true},
		{name: "get-state removed", command: "get-state", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subject, send, err := vpnCommandSubject(tt.command)
			if tt.wantErr {
				if err == nil {
					t.Fatal("err = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if subject != tt.wantSubject {
				t.Fatalf("subject = %q, want %q", subject, tt.wantSubject)
			}
			if send != tt.wantSend {
				t.Fatalf("send = %t, want %t", send, tt.wantSend)
			}
		})
	}
}

func TestVPNStatusJSONContract(t *testing.T) {
	activeService := serviceActivity{active: true, state: "active"}
	inactiveService := serviceActivity{active: false, state: "inactive"}

	status := classifyAtmosInterfaceStatus(net.FlagUp, []string{"100.65.0.1/32"}, activeService)
	assertJSON(t, status, `{"schemaVersion":1,"state":"connected","interface":"atmos","addresses":["100.65.0.1/32"],"service":"atmos-agent.service","serviceActive":true,"serviceState":"active"}`)

	status = newVPNStatus("disconnected", nil, reasonInterfaceMissing, inactiveService)
	assertJSON(t, status, `{"schemaVersion":1,"state":"disconnected","interface":"atmos","addresses":[],"reason":"interface_missing","service":"atmos-agent.service","serviceActive":false,"serviceState":"inactive"}`)
}

func TestFormatVPNStatusText(t *testing.T) {
	activeService := serviceActivity{active: true, state: "active"}
	inactiveService := serviceActivity{active: false, state: "inactive"}
	failedService := serviceActivity{active: false, state: "failed"}

	tests := []struct {
		name   string
		status vpnStatus
		want   string
	}{
		{
			name:   "connected",
			status: classifyAtmosInterfaceStatus(net.FlagUp, []string{"100.65.0.1/32"}, activeService),
			want:   "connected\tservice active, addresses 100.65.0.1/32",
		},
		{
			name:   "missing with active service",
			status: newVPNStatus("disconnected", nil, reasonInterfaceMissing, activeService),
			want:   "disconnected\tservice active, interface missing",
		},
		{
			name:   "missing with inactive service",
			status: newVPNStatus("disconnected", nil, reasonInterfaceMissing, inactiveService),
			want:   "agent-inactive\tservice inactive, interface missing",
		},
		{
			name:   "missing with failed service",
			status: newVPNStatus("disconnected", nil, reasonInterfaceMissing, failedService),
			want:   "agent-failed\tservice failed, interface missing",
		},
		{
			name:   "down",
			status: classifyAtmosInterfaceStatus(0, []string{"100.65.0.1/32"}, activeService),
			want:   "disconnected\tservice active, interface down",
		},
		{
			name:   "unknown",
			status: classifyAtmosInterfaceStatus(net.FlagUp, []string{"10.0.0.1/32"}, activeService),
			want:   "unknown\tservice active, addresses 10.0.0.1/32",
		},
		{
			name:   "unknown with inactive service",
			status: classifyAtmosInterfaceStatus(net.FlagUp, []string{"10.0.0.1/32"}, inactiveService),
			want:   "agent-inactive\tservice inactive, addresses 10.0.0.1/32",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatVPNStatusText(tt.status); got != tt.want {
				t.Fatalf("formatVPNStatusText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseServiceActiveOutput(t *testing.T) {
	tests := []struct {
		name       string
		output     string
		err        error
		wantActive bool
		wantState  string
		wantErr    bool
	}{
		{name: "active", output: "active\n", wantActive: true, wantState: "active"},
		{name: "inactive", output: "inactive\n", err: errCommandFailed, wantState: "inactive"},
		{name: "failed", output: "failed\n", err: errCommandFailed, wantState: "failed"},
		{name: "unknown empty", err: errCommandFailed, wantState: "unknown"},
		{name: "bus error", output: "Failed to connect to bus\n", err: errCommandFailed, wantState: "Failed to connect to bus", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, err := parseServiceActiveOutput(tt.output, tt.err)
			if tt.wantErr {
				if err == nil {
					t.Fatal("err = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if status.active != tt.wantActive {
				t.Fatalf("active = %t, want %t", status.active, tt.wantActive)
			}
			if status.state != tt.wantState {
				t.Fatalf("state = %q, want %q", status.state, tt.wantState)
			}
		})
	}
}

func TestDesktopEntryHidden(t *testing.T) {
	tests := []struct {
		name string
		data string
		want bool
	}{
		{name: "true", data: "Hidden=true\n", want: true},
		{name: "true with spacing and case", data: "Hidden = TRUE\n", want: true},
		{name: "false", data: "Hidden=false\n"},
		{name: "absent", data: "Type=Application\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := desktopEntryHidden(tt.data); got != tt.want {
				t.Fatalf("desktopEntryHidden() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestFormatGuiAutostartStatus(t *testing.T) {
	tests := []struct {
		name           string
		overrideHidden bool
		serviceEnabled bool
		wantEnabled    bool
		wantDetail     string
	}{
		{
			name:           "suppressed with service enabled",
			overrideHidden: true,
			serviceEnabled: true,
			wantDetail:     "GUI login launch suppressed, user service enabled",
		},
		{
			name:           "suppressed without service enabled",
			overrideHidden: true,
			wantDetail:     "GUI login launch suppressed, user service disabled",
		},
		{
			name:           "enabled with service enabled",
			serviceEnabled: true,
			wantEnabled:    true,
			wantDetail:     "GUI login launch enabled, user service enabled",
		},
		{
			name:        "default enabled",
			wantEnabled: true,
			wantDetail:  "GUI login launch enabled, user service disabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enabled, detail := formatGuiAutostartStatus(tt.overrideHidden, tt.serviceEnabled)
			if enabled != tt.wantEnabled {
				t.Fatalf("enabled = %t, want %t", enabled, tt.wantEnabled)
			}
			if detail != tt.wantDetail {
				t.Fatalf("detail = %q, want %q", detail, tt.wantDetail)
			}
		})
	}
}

func TestGuiAutostartStatusJSONContract(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)

	status := buildGuiAutostartStatus(false, false)
	want := `{"schemaVersion":1,"enabled":true,"overrideHidden":false,"serviceEnabled":false,"overridePath":"` +
		filepath.Join(configDir, "autostart", atmosAutostartFilename) +
		`","service":"atmos-agent.service"}`
	assertJSON(t, status, want)
}

func TestVersionAndCommandJSONContracts(t *testing.T) {
	assertJSON(t, versionOutput{
		SchemaVersion: schemaVersion,
		Version:       version,
	}, `{"schemaVersion":1,"version":"`+version+`"}`)

	assertJSON(t, commandResult{
		SchemaVersion: schemaVersion,
		OK:            true,
		Command:       "vpn.pause",
	}, `{"schemaVersion":1,"ok":true,"command":"vpn.pause"}`)
}

func TestWriteGuiAutostartSuppressOverride(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)

	if err := writeGuiAutostartSuppressOverride(); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(configDir, "autostart", atmosAutostartFilename)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	for _, want := range []string{
		"[Desktop Entry]\n",
		"Type=Application\n",
		"Name=Atmos Agent\n",
		"Exec=/usr/bin/atmos\n",
		"Terminal=false\n",
		"Hidden=true\n",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("desktop entry missing %q in:\n%s", want, content)
		}
	}
	if !desktopEntryHidden(content) {
		t.Fatal("written desktop entry is not hidden")
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Fatalf("mode = %v, want 0644", got)
	}
}

func assertJSON(t *testing.T, value any, want string) {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); got != want {
		t.Fatalf("json = %s, want %s", got, want)
	}
}

var errCommandFailed = errors.New("command failed")
