package main

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"
)

// discord talks to the Discord desktop app over its local IPC pipe/socket.
// Each frame is: opcode (uint32 LE), length (uint32 LE), JSON body.
type discord struct {
	io.ReadWriteCloser
	clientID string
}

func dial(clientID string) (*discord, error) {
	err := errors.New("Discord is not running")
	for _, p := range ipcPaths() {
		c, e := dialPipe(p)
		if e != nil {
			continue
		}
		d := &discord{c, clientID}
		if err = d.send(0, map[string]any{"v": 1, "client_id": clientID}); err == nil {
			if err = d.recv(); err == nil {
				return d, nil
			}
		}
		c.Close()
	}
	return nil, err
}

func (d *discord) setActivity(a map[string]any) error {
	err := d.send(1, map[string]any{
		"cmd":   "SET_ACTIVITY",
		"args":  map[string]any{"pid": os.Getpid(), "activity": a},
		"nonce": strconv.FormatInt(time.Now().UnixNano(), 10),
	})
	if err != nil {
		return err
	}
	return d.recv()
}

func (d *discord) send(op uint32, v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	frame := binary.LittleEndian.AppendUint32(nil, op)
	frame = binary.LittleEndian.AppendUint32(frame, uint32(len(body)))
	_, err = d.Write(append(frame, body...))
	return err
}

func (d *discord) recv() error {
	var h [8]byte
	if _, err := io.ReadFull(d, h[:]); err != nil {
		return err
	}
	n := binary.LittleEndian.Uint32(h[4:])
	if n > 1<<20 {
		return fmt.Errorf("frame too large: %d bytes", n)
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(d, body); err != nil {
		return err
	}
	var r struct {
		Evt     string
		Message string
		Data    struct{ Message string }
	}
	json.Unmarshal(body, &r)
	if binary.LittleEndian.Uint32(h[:4]) == 2 { // CLOSE, e.g. a bad client_id
		return errors.New("Discord closed the connection: " + r.Message)
	}
	if r.Evt == "ERROR" {
		return errors.New(r.Data.Message)
	}
	return nil
}
