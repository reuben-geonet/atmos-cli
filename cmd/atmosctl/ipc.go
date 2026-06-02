package main

import (
	"encoding/json"
	"fmt"
	"net"
	"time"
)

const (
	subjectStart = "tunnel.Start"
	subjectStop  = "tunnel.Stop"
)

type message struct {
	Subject         string          `json:"subject,omitempty"`
	ReplyID         string          `json:"replyID,omitempty"`
	RequestID       string          `json:"RequestID,omitempty"`
	PayloadRaw      json.RawMessage `json:"payloadRaw,omitempty"`
	PayloadIsStream bool            `json:"payloadIsStream"`
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
