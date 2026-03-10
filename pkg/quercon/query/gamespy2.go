package query

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/text/encoding/charmap"
)

const (
	gamespy2QueryHeader     = "\xFE\xFD\x00"
	gamespy2SessionID       = "\x10\x20\x30\x40"
	gamespy2FullInfoFlags   = "\xFF\xFF\xFF"
	gamespy2MaxPacketSize   = 4096
	gamespy2ResponseMinSize = 5
)

func queryGameSpy2(ctx context.Context, host string, port int) (*Result, error) {
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

	queryPacket := []byte(gamespy2QueryHeader + gamespy2SessionID + gamespy2FullInfoFlags)
	_, err = conn.Write(queryPacket)
	if err != nil {
		return result, errors.Wrap(err, "failed to send query packet")
	}

	buffer := make([]byte, gamespy2MaxPacketSize)
	n, err := conn.Read(buffer)
	if err != nil {
		return result, errors.Wrap(err, "failed to read response")
	}

	responseData := buffer[:n]

	err = parseGameSpy2Response(responseData, result)
	if err != nil {
		return result, errors.Wrap(err, "failed to parse response")
	}

	result.Online = true

	return result, nil
}

func parseGameSpy2Response(data []byte, result *Result) error {
	if len(data) < gamespy2ResponseMinSize {
		return errors.New("response too short")
	}

	if data[0] != 0x00 {
		return errors.Errorf("invalid response type: expected 0x00, got 0x%02x", data[0])
	}

	offset := 5

	if len(data) <= offset {
		return nil
	}

	reader := bytes.NewReader(data[offset:])
	parseGameSpy2ServerDetails(reader, result)
	parseGameSpy2Players(reader, result)

	return nil
}

func parseGameSpy2ServerDetails(reader *bytes.Reader, result *Result) {
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
		//nolint:goconst
		switch keyLower {
		case "hostname", "sv_hostname", "servername":
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

func parseGameSpy2Players(reader *bytes.Reader, result *Result) {
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
