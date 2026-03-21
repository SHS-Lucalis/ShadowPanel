package query

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/text/encoding/charmap"
)

const (
	quake3QueryPacket    = "\xFF\xFF\xFF\xFFgetstatus\n"
	quake3ResponseHeader = "\xFF\xFF\xFF\xFFstatusResponse\n"
	quake3MaxPacketSize  = 4096
)

var quake3PlayerRegex = regexp.MustCompile(`^(-?\d+)\s+(\d+)\s+"(.*)"$`)

func queryQuake3(ctx context.Context, host string, port int) (*Result, error) {
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

	_, err = conn.Write([]byte(quake3QueryPacket))
	if err != nil {
		return result, errors.Wrap(err, "failed to send query packet")
	}

	buffer := make([]byte, quake3MaxPacketSize)
	n, err := conn.Read(buffer)
	if err != nil {
		return result, errors.Wrap(err, "failed to read query response")
	}

	response := buffer[:n]

	err = parseQuake3Response(response, result)
	if err != nil {
		return result, errors.Wrap(err, "failed to parse response")
	}

	result.Online = true

	return result, nil
}

func parseQuake3Response(data []byte, result *Result) error {
	headerLen := len(quake3ResponseHeader)
	if len(data) < headerLen {
		return errors.New("response too short")
	}

	if !bytes.HasPrefix(data, []byte(quake3ResponseHeader)) {
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

func parseQuake3ServerVars(data []byte, result *Result) {
	decoder := charmap.ISO8859_1.NewDecoder()

	parts := strings.Split(string(data), "\\")
	if len(parts) < 2 {
		return
	}

	// Format: \key1\value1\key2\value2...
	// First element is empty (before first \)
	for i := 1; i+1 < len(parts); i += 2 {
		key := strings.ToLower(parts[i])
		value := parts[i+1]

		valueBytes, err := decoder.Bytes([]byte(value))
		valueUTF8 := value
		if err == nil {
			valueUTF8 = string(valueBytes)
		}

		switch key {
		case "sv_hostname", "hostname":
			result.Name = valueUTF8
		case "mapname":
			result.Map = valueUTF8
		case "sv_maxclients", "maxclients":
			result.MaxPlayersNum, _ = strconv.Atoi(valueUTF8)
		}
	}
}

func parseQuake3Players(lines [][]byte, result *Result) {
	decoder := charmap.ISO8859_1.NewDecoder()
	players := make([]ResultPlayer, 0, len(lines))

	for _, line := range lines {
		lineStr := strings.TrimSpace(string(line))
		if lineStr == "" {
			continue
		}

		matches := quake3PlayerRegex.FindStringSubmatch(lineStr)
		if matches == nil {
			continue
		}

		score, _ := strconv.Atoi(matches[1])
		name := matches[3]

		nameBytes, err := decoder.Bytes([]byte(name))
		nameUTF8 := name
		if err == nil {
			nameUTF8 = string(nameBytes)
		}

		players = append(players, ResultPlayer{
			Name:  nameUTF8,
			Score: score,
		})
	}

	result.Players = players
	result.PlayersNum = len(players)
}
