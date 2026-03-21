package query

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

const (
	raknetUnconnectedPing = 0x01
	raknetUnconnectedPong = 0x1C
	raknetMaxPacketSize   = 4096
)

var raknetMagic = []byte{
	0x00, 0xFF, 0xFF, 0x00,
	0xFE, 0xFE, 0xFE, 0xFE,
	0xFD, 0xFD, 0xFD, 0xFD,
	0x12, 0x34, 0x56, 0x78,
}

func queryRakNet(ctx context.Context, host string, port int) (*Result, error) {
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

	packet := buildRakNetPingPacket()
	_, err = conn.Write(packet)
	if err != nil {
		return result, errors.Wrap(err, "failed to send ping packet")
	}

	buffer := make([]byte, raknetMaxPacketSize)
	n, err := conn.Read(buffer)
	if err != nil {
		return result, errors.Wrap(err, "failed to read response")
	}

	response := buffer[:n]

	err = parseRakNetResponse(response, result)
	if err != nil {
		return result, errors.Wrap(err, "failed to parse response")
	}

	result.Online = true

	return result, nil
}

func buildRakNetPingPacket() []byte {
	packet := make([]byte, 25)

	packet[0] = raknetUnconnectedPing

	timestamp := time.Now().UnixMilli()
	// #nosec G115 - timestamp is always positive for current time
	binary.BigEndian.PutUint64(packet[1:9], uint64(timestamp))

	copy(packet[9:25], raknetMagic)

	return packet
}

func parseRakNetResponse(data []byte, result *Result) error {
	// Minimum response size: 1 (type) + 8 (timestamp) + 8 (guid) + 16 (magic) + 2 (string length) = 35
	if len(data) < 35 {
		return errors.New("response too short")
	}

	if data[0] != raknetUnconnectedPong {
		return errors.Errorf("invalid packet type: expected 0x%02x, got 0x%02x", raknetUnconnectedPong, data[0])
	}

	// Validate magic at offset 17 (after type + timestamp + guid)
	if !bytes.Equal(data[17:33], raknetMagic) {
		return errors.New("invalid magic bytes")
	}

	stringLength := binary.BigEndian.Uint16(data[33:35])
	if len(data) < 35+int(stringLength) {
		return errors.Errorf("response too short for payload: need %d, have %d", 35+int(stringLength), len(data))
	}

	payload := string(data[35 : 35+stringLength])

	return parseRakNetPayload(payload, result)
}

func parseRakNetPayload(payload string, result *Result) error {
	// Format: Edition;ServerName;ProtocolVersion;VersionName;Players;MaxPlayers;ServerID;LevelName;GameMode;...
	parts := strings.Split(payload, ";")

	if len(parts) < 6 {
		return errors.Errorf("invalid payload format: expected at least 6 parts, got %d", len(parts))
	}

	// parts[0] = Edition (MCPE/MCBE)
	result.Name = parts[1]
	// parts[2] = Protocol version
	// parts[3] = Version name

	if players, err := strconv.Atoi(parts[4]); err == nil {
		result.PlayersNum = players
	}

	if maxPlayers, err := strconv.Atoi(parts[5]); err == nil {
		result.MaxPlayersNum = maxPlayers
	}

	// parts[6] = Server ID
	// parts[7] = Level name (if present)
	if len(parts) > 8 {
		result.Map = parts[8] // Game mode as map
	}

	return nil
}
