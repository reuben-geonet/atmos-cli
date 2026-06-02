package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type versionOutput struct {
	SchemaVersion int    `json:"schemaVersion"`
	Version       string `json:"version"`
}

type commandResult struct {
	SchemaVersion int    `json:"schemaVersion"`
	OK            bool   `json:"ok"`
	Command       string `json:"command"`
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
