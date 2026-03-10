package query

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"time"

	"github.com/pkg/errors"
)

const (
	sampHeader        = "SAMP"
	sampInfoOpcode    = 'i'
	sampMaxPacketSize = 4096
)

func querySAMP(ctx context.Context, host string, port int) (*Result, error) {
	result := &Result{
		Online:    false,
		QueryTime: time.Now(),
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return result, errors.New("SAMP protocol requires IP address, not hostname")
	}

	ipv4 := ip.To4()
	if ipv4 == nil {
		return result, errors.New("SAMP protocol only supports IPv4 addresses")
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

	packet := buildSAMPPacket(ipv4, port, sampInfoOpcode)
	_, err = conn.Write(packet)
	if err != nil {
		return result, errors.Wrap(err, "failed to send query packet")
	}

	buffer := make([]byte, sampMaxPacketSize)
	n, err := conn.Read(buffer)
	if err != nil {
		return result, errors.Wrap(err, "failed to read response")
	}

	response := buffer[:n]

	err = parseSAMPResponse(response, packet[:11], result)
	if err != nil {
		return result, errors.Wrap(err, "failed to parse response")
	}

	result.Online = true

	return result, nil
}

func buildSAMPPacket(ip net.IP, port int, opcode byte) []byte {
	// Packet structure: "SAMP" (4) + IP (4) + Port (2 LE) + Opcode (1) = 11 bytes
	packet := make([]byte, 11)

	copy(packet[0:4], sampHeader)

	copy(packet[4:8], ip)

	// #nosec G115 - port is validated to be a valid network port (0-65535)
	binary.LittleEndian.PutUint16(packet[8:10], uint16(port))

	packet[10] = opcode

	return packet
}

func parseSAMPResponse(data []byte, expectedHeader []byte, result *Result) error {
	// Minimum: 11 (header) + 1 (password) + 2 (players) + 2 (maxplayers) + 4 (hostname len) = 20
	if len(data) < 20 {
		return errors.New("response too short")
	}

	if !bytes.Equal(data[:11], expectedHeader) {
		return errors.New("invalid response header")
	}

	reader := bytes.NewReader(data[11:])

	// Password flag (1 byte)
	_, err := reader.ReadByte()
	if err != nil {
		return errors.Wrap(err, "failed to read password flag")
	}

	// Players count (2 bytes LE)
	var numPlayers uint16
	err = binary.Read(reader, binary.LittleEndian, &numPlayers)
	if err != nil {
		return errors.Wrap(err, "failed to read players count")
	}
	result.PlayersNum = int(numPlayers)

	// Max players (2 bytes LE)
	var maxPlayers uint16
	err = binary.Read(reader, binary.LittleEndian, &maxPlayers)
	if err != nil {
		return errors.Wrap(err, "failed to read max players")
	}
	result.MaxPlayersNum = int(maxPlayers)

	// Hostname (4-byte length prefix + string)
	hostname, err := readSAMPString(reader)
	if err != nil {
		return errors.Wrap(err, "failed to read hostname")
	}
	result.Name = hostname

	// Gametype (4-byte length prefix + string)
	gametype, err := readSAMPString(reader)
	if err != nil {
		return errors.Wrap(err, "failed to read gametype")
	}
	result.Map = gametype

	return nil
}

func readSAMPString(reader *bytes.Reader) (string, error) {
	var length uint32
	err := binary.Read(reader, binary.LittleEndian, &length)
	if err != nil {
		return "", errors.Wrap(err, "failed to read string length")
	}

	if length > sampMaxPacketSize {
		return "", errors.Errorf("string length too large: %d", length)
	}

	if length == 0 {
		return "", nil
	}

	data := make([]byte, length)
	n, err := reader.Read(data)
	if err != nil {
		return "", errors.Wrap(err, "failed to read string data")
	}

	// #nosec G115 - n is from a read operation with length < sampMaxPacketSize
	if uint32(n) < length {
		return "", errors.Errorf("short read: expected %d bytes, got %d", length, n)
	}

	return string(data), nil
}
