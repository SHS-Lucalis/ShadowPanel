package query

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/text/encoding/charmap"
)

const (
	gamespy3ChallengePacket = "\xFE\xFD\x09\x10\x20\x30\x40"
	gamespy3QueryPacketFmt  = "\xFE\xFD\x00\x10\x20\x30\x40%s\xFF\xFF\xFF\x01"
	gamespy3MaxPacketSize   = 4096
	gamespy3SplitMarker     = "splitnum\x00"
)

func queryGameSpy3(ctx context.Context, host string, port int) (*Result, error) {
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

	_, err = conn.Write([]byte(gamespy3ChallengePacket))
	if err != nil {
		return result, errors.Wrap(err, "failed to send challenge packet")
	}

	challengeResponse := make([]byte, gamespy3MaxPacketSize)
	n, err := conn.Read(challengeResponse)
	if err != nil {
		return result, errors.Wrap(err, "failed to read challenge response")
	}
	challengeResponse = challengeResponse[:n]

	challenge, err := parseGameSpy3Challenge(challengeResponse)
	if err != nil {
		return result, errors.Wrap(err, "failed to parse challenge")
	}

	queryPacket := fmt.Sprintf(gamespy3QueryPacketFmt, string(challenge))
	_, err = conn.Write([]byte(queryPacket))
	if err != nil {
		return result, errors.Wrap(err, "failed to send query packet")
	}

	packets, err := readGameSpy3Packets(conn, deadline)
	if err != nil {
		return result, errors.Wrap(err, "failed to read response packets")
	}

	responseData := cleanGameSpy3Packets(packets)

	if len(responseData) == 0 {
		return result, errors.New("no data in query response")
	}

	err = parseGameSpy3Response(responseData, result)
	if err != nil {
		return result, errors.Wrap(err, "failed to parse response")
	}

	result.Online = true

	return result, nil
}

func parseGameSpy3Challenge(response []byte) ([]byte, error) {
	if len(response) < 5 {
		return nil, errors.New("challenge response too short")
	}

	challengeStr := string(response[5:])
	challengeStr = strings.TrimRight(challengeStr, "\x00")

	if challengeStr == "" || challengeStr == "0" {
		return make([]byte, 4), nil
	}

	challengeNum, err := strconv.ParseInt(challengeStr, 10, 64)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse challenge number")
	}

	challenge := make([]byte, 4)
	// #nosec G115 - challengeNum is from server response, overflow is acceptable
	binary.BigEndian.PutUint32(challenge, uint32(challengeNum))

	return challenge, nil
}

func readGameSpy3Packets(conn net.Conn, deadline time.Time) ([]gamespy3Packet, error) {
	var packets []gamespy3Packet

	for time.Now().Before(deadline) {
		buffer := make([]byte, gamespy3MaxPacketSize)
		n, err := conn.Read(buffer)
		if err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				break
			}

			if len(packets) > 0 {
				break
			}

			return nil, errors.Wrap(err, "failed to read packet")
		}

		data := buffer[:n]

		packet, err := parseGameSpy3PacketHeader(data)
		if err != nil {
			continue
		}

		packets = append(packets, packet)

		if !packet.isSplit {
			break
		}

		shortTimeout := 100 * time.Millisecond
		_ = conn.SetReadDeadline(time.Now().Add(shortTimeout))
	}

	return packets, nil
}

type gamespy3Packet struct {
	packetID int
	data     []byte
	isSplit  bool
}

func parseGameSpy3PacketHeader(data []byte) (gamespy3Packet, error) {
	// Minimum: type (1) + session_id (4) = 5 bytes
	if len(data) < 5 {
		return gamespy3Packet{}, errors.New("packet too short")
	}

	if data[0] != 0x00 {
		return gamespy3Packet{}, errors.Errorf("invalid packet type: expected 0x00, got 0x%02x", data[0])
	}

	// Check for split packet marker after session ID
	offset := 5
	markerLen := len(gamespy3SplitMarker)
	hasSplitMarker := len(data) > offset+markerLen &&
		string(data[offset:offset+markerLen]) == gamespy3SplitMarker
	if hasSplitMarker {
		offset += markerLen

		if len(data) <= offset+1 {
			return gamespy3Packet{}, errors.New("split packet header incomplete")
		}

		packetID := int(data[offset] & 0x7F)
		offset += 2 // packet_id + unknown byte

		return gamespy3Packet{
			packetID: packetID,
			data:     data[offset:],
			isSplit:  true,
		}, nil
	}

	// Non-split packet: skip 16 bytes header (type + session_id + padding)
	headerSize := 16
	if len(data) <= headerSize {
		return gamespy3Packet{
			packetID: 0,
			data:     nil,
			isSplit:  false,
		}, nil
	}

	return gamespy3Packet{
		packetID: 0,
		data:     data[headerSize:],
		isSplit:  false,
	}, nil
}

func cleanGameSpy3Packets(packets []gamespy3Packet) []byte {
	if len(packets) == 0 {
		return nil
	}

	if len(packets) == 1 {
		return packets[0].data
	}

	sort.Slice(packets, func(i, j int) bool {
		return packets[i].packetID < packets[j].packetID
	})

	var result []byte
	var prevEnding []byte

	for _, packet := range packets {
		data := packet.data
		if len(data) == 0 {
			continue
		}

		if len(prevEnding) > 0 && len(data) > 0 {
			overlapLen := findOverlap(prevEnding, data)
			if overlapLen > 0 {
				data = data[overlapLen:]
			}
		}

		result = append(result, data...)

		if len(result) > 50 {
			prevEnding = result[len(result)-50:]
		} else {
			prevEnding = result
		}
	}

	return result
}

func findOverlap(ending, beginning []byte) int {
	maxCheck := min(len(ending), len(beginning))

	for i := maxCheck; i > 0; i-- {
		if bytes.Equal(ending[len(ending)-i:], beginning[:i]) {
			return i
		}
	}

	return 0
}

func parseGameSpy3Response(data []byte, result *Result) error {
	if len(data) == 0 {
		return errors.New("empty response data")
	}

	sections := bytes.Split(data, []byte{0x00, 0x00, 0x01})

	if len(sections) == 0 {
		return errors.New("invalid response format")
	}

	parseGameSpy3ServerDetails(sections[0], result)

	if len(sections) > 1 && len(sections[1]) > 0 {
		parseGameSpy3Players(sections[1], result)
	}

	return nil
}

func parseGameSpy3ServerDetails(data []byte, result *Result) {
	reader := bytes.NewReader(data)
	decoder := charmap.ISO8859_1.NewDecoder()

	for {
		key, err := readNullTerminatedString(reader)
		if err != nil || key == "" {
			break
		}

		value, err := readNullTerminatedString(reader)
		if err != nil {
			break
		}

		valueBytes, err := decoder.Bytes([]byte(value))
		valueUTF8 := value
		if err == nil {
			valueUTF8 = string(valueBytes)
		}

		keyLower := strings.ToLower(key)
		switch keyLower {
		case "hostname", "sv_hostname":
			result.Name = valueUTF8
		case "mapname", "map":
			result.Map = valueUTF8
		case "numplayers":
			result.PlayersNum, _ = strconv.Atoi(valueUTF8)
		case "maxplayers":
			result.MaxPlayersNum, _ = strconv.Atoi(valueUTF8)
		}
	}
}

func parseGameSpy3Players(data []byte, result *Result) {
	reader := bytes.NewReader(data)
	decoder := charmap.ISO8859_1.NewDecoder()

	var playerNames []string
	var playerScores []int

	for {
		fieldHeader, err := readNullTerminatedString(reader)
		if err != nil || fieldHeader == "" {
			break
		}

		fieldHeaderLower := strings.ToLower(fieldHeader)

		fieldType, hasSuffix := strings.CutSuffix(fieldHeaderLower, "_")
		if hasSuffix {
			for {
				value, err := readNullTerminatedString(reader)
				if err != nil {
					break
				}
				if value == "" {
					break
				}

				valueBytes, _ := decoder.Bytes([]byte(value))
				valueUTF8 := string(valueBytes)

				switch fieldType {
				case "player":
					playerNames = append(playerNames, valueUTF8)
				case "score":
					score, _ := strconv.Atoi(valueUTF8)
					playerScores = append(playerScores, score)
				}
			}
		}
	}

	if len(playerNames) == 0 {
		return
	}

	result.Players = make([]ResultPlayer, len(playerNames))
	for i, name := range playerNames {
		result.Players[i].Name = name
		if i < len(playerScores) {
			result.Players[i].Score = playerScores[i]
		}
	}

	if result.PlayersNum == 0 {
		result.PlayersNum = len(playerNames)
	}
}
