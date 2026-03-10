package query

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"time"

	"github.com/pkg/errors"
)

const (
	quake2QueryPacket    = "\xFF\xFF\xFF\xFFstatus\x00"
	quake2ResponseHeader = "\xFF\xFF\xFF\xFFprint\n"
	quake2MaxPacketSize  = 4096
)

func queryQuake2(ctx context.Context, host string, port int) (*Result, error) {
	result := &Result{
		Online:    false,
		QueryTime: time.Now(),
	}

	address := fmt.Sprintf("%s:%d", host, port)

	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "udp", address)
	if err != nil {
		return result, errors.Wrap(err, "failed to create UDP connection")
	}
	defer func() {
		_ = conn.Close()
	}()

	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(defaultTimeout)
	}
	err = conn.SetDeadline(deadline)
	if err != nil {
		return result, errors.Wrap(err, "failed to set deadline")
	}

	_, err = conn.Write([]byte(quake2QueryPacket))
	if err != nil {
		return result, errors.Wrap(err, "failed to send query packet")
	}

	buffer := make([]byte, quake2MaxPacketSize)
	n, err := conn.Read(buffer)
	if err != nil {
		return result, errors.Wrap(err, "failed to read query response")
	}

	response := buffer[:n]

	err = parseQuake2Response(response, result)
	if err != nil {
		return result, errors.Wrap(err, "failed to parse response")
	}

	result.Online = true

	return result, nil
}

func parseQuake2Response(data []byte, result *Result) error {
	headerLen := len(quake2ResponseHeader)
	if len(data) < headerLen {
		return errors.New("response too short")
	}

	if !bytes.HasPrefix(data, []byte(quake2ResponseHeader)) {
		return errors.New("invalid response header")
	}

	responseData := data[headerLen:]

	lines := bytes.Split(responseData, []byte("\n"))
	if len(lines) == 0 {
		return errors.New("empty response")
	}

	if len(lines[0]) > 0 {
		parseQuake3ServerVars(lines[0], result)
	}

	if len(lines) > 1 {
		parseQuake3Players(lines[1:], result)
	}

	return nil
}
