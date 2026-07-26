package main

import (
	"path/filepath"
	"testing"

	"github.com/syndtr/goleveldb/leveldb"
)

func TestReadAgentLoggedIn(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{
			name:  "logged in",
			value: `{"login":true,"loginState":"LoggedIn"}`,
			want:  true,
		},
		{
			name:  "logged out",
			value: `{"login":false,"loginState":"LoggedOut"}`,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			databasePath := filepath.Join(t.TempDir(), "leveldb")
			database, err := leveldb.OpenFile(databasePath, nil)
			if err != nil {
				t.Fatal(err)
			}
			key := append([]byte("_file://\x00\x01"), []byte(agentStateCacheKey)...)
			if err := database.Put(key, []byte(tt.value), nil); err != nil {
				t.Fatal(err)
			}
			if err := database.Close(); err != nil {
				t.Fatal(err)
			}

			got, err := readAgentLoggedIn(databasePath)
			if err != nil {
				t.Fatal(err)
			}
			if got == nil || *got != tt.want {
				t.Fatalf("loggedIn = %v, want %t", got, tt.want)
			}
		})
	}
}

func TestParseCachedAgentStateRejectsUnknownAndInconsistentStates(t *testing.T) {
	tests := []string{
		`{"login":true,"loginState":"LoggedOut"}`,
		`{"login":false,"loginState":"LoggedIn"}`,
		`{"login":false,"loginState":"Unknown"}`,
		`{"loginState":"LoggedOut"}`,
		`not JSON`,
	}

	for _, value := range tests {
		if _, err := parseCachedAgentState([]byte(value)); err == nil {
			t.Fatalf("parseCachedAgentState(%q) error = nil", value)
		}
	}
}
