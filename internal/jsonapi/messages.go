package jsonapi

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

type messageType struct {
	Type string `json:"type"`
}

type queryMessage struct {
	Query string `json:"query"`
}

type queryHostMessage struct {
	Host string `json:"host"`
}

type getLoginMessage struct {
	Entry string `json:"entry"`
}

type copyToClipboard struct {
	Entry string `json:"entry"`
	Key   string `json:"key"`
}

type loginResponse struct {
	Username    string                 `json:"username"`
	Password    string                 `json:"password"`
	LoginFields map[string]interface{} `json:"login_fields,omitempty"`
}

type getDataMessage struct {
	Entry string `json:"entry"`
}

type getVersionMessage struct {
	Version string `json:"version"`
	Major   uint64 `json:"major"`
	Minor   uint64 `json:"minor"`
	Patch   uint64 `json:"patch"`
}

type createEntryMessage struct {
	Name           string `json:"entry_name"`
	Login          string `json:"login"`
	Password       string `json:"password"`
	PasswordLength int    `json:"length"`
	Generate       bool   `json:"generate"`
	UseSymbols     bool   `json:"use_symbols"`
}

type statusResponse struct {
	Status string `json:"status"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func readRequest(r io.Reader) ([]byte, error) {
	input := bufio.NewReader(r)
	lenBytes := make([]byte, 4)
	count, err := input.Read(lenBytes)
	if err != nil {
		return nil, eofReturn(err)
	}
	if count != 4 {
		return nil, fmt.Errorf("not enough bytes read to determine message size")
	}

	length, err := getMessageLength(lenBytes)
	if err != nil {
		return nil, err
	}

	msgBytes := make([]byte, length)
	count, err = input.Read(msgBytes)
	if err != nil {
		return nil, eofReturn(err)
	}

	if count != length {
		return nil, fmt.Errorf("incomplete message read")
	}

	return msgBytes, nil
}

func getMessageLength(msg []byte) (int, error) {
	var length uint32
	buf := bytes.NewBuffer(msg)
	if err := binary.Read(buf, binary.LittleEndian, &length); err != nil {
		return 0, err
	}

	return int(length), nil
}

func eofReturn(err error) error {
	if errors.Is(err, io.EOF) {
		return nil
	}

	return err
}

func sendResponse(msg any, w io.Writer) error {
	// we can't use json.NewEncoder(w).Encode because we need to send the final
	// message length before the actual JSON
	buf, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	if err := writeMessageLength(buf, w); err != nil {
		return err
	}

	n, err := w.Write(buf)
	if err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}
	if n != len(buf) {
		return fmt.Errorf("message not fully written (wrote %d of %d)", n, len(buf))
	}

	return nil
}

func writeMessageLength(msg []byte, w io.Writer) error {
	return binary.Write(w, binary.LittleEndian, uint32(len(msg)))
}
