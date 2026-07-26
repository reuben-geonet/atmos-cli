package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

const (
	axisClientConfigDir = "Axis Client"
	agentStateCacheKey  = "agent.state.local"
)

type cachedAgentState struct {
	Login      *bool  `json:"login"`
	LoginState string `json:"loginState"`
}

func agentLoggedIn() *bool {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil
	}

	loggedIn, err := readAgentLoggedIn(filepath.Join(
		configDir,
		axisClientConfigDir,
		"Local Storage",
		"leveldb",
	))
	if err != nil {
		return nil
	}
	return loggedIn
}

func readAgentLoggedIn(databasePath string) (*bool, error) {
	database, err := leveldb.OpenFile(databasePath, &opt.Options{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("open Atmos agent-state cache: %w", err)
	}
	defer database.Close()

	iterator := database.NewIterator(nil, nil)
	defer iterator.Release()

	for iterator.Next() {
		if !bytes.HasSuffix(iterator.Key(), []byte(agentStateCacheKey)) {
			continue
		}

		loggedIn, err := parseCachedAgentState(iterator.Value())
		if err != nil {
			return nil, err
		}
		return loggedIn, nil
	}
	if err := iterator.Error(); err != nil {
		return nil, fmt.Errorf("read Atmos agent-state cache: %w", err)
	}

	return nil, errors.New("Atmos agent-state cache entry not found")
}

func parseCachedAgentState(value []byte) (*bool, error) {
	jsonStart := bytes.IndexByte(value, '{')
	if jsonStart < 0 {
		return nil, errors.New("Atmos agent-state cache value is not JSON")
	}

	var state cachedAgentState
	if err := json.Unmarshal(value[jsonStart:], &state); err != nil {
		return nil, fmt.Errorf("decode Atmos agent-state cache: %w", err)
	}
	if state.Login == nil {
		return nil, errors.New("Atmos agent-state cache has no login value")
	}

	switch state.LoginState {
	case "LoggedIn":
		if !*state.Login {
			return nil, errors.New("Atmos agent-state cache has inconsistent logged-in state")
		}
	case "LoggedOut":
		if *state.Login {
			return nil, errors.New("Atmos agent-state cache has inconsistent logged-out state")
		}
	default:
		return nil, fmt.Errorf("unsupported Atmos login state %q", state.LoginState)
	}

	return state.Login, nil
}
