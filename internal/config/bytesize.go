package config

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

// ByteSize is a uint64-based byte count that accepts human-readable suffixes
// (B, K/KB/KiB, M/MB/MiB, G/GB/GiB, T/TB/TiB, P/PB/PiB) when unmarshalled
// from text. All suffixes are interpreted as powers of 1024 (binary).
// Plain integers without a suffix are interpreted as raw bytes.
type ByteSize uint64

func (b ByteSize) Uint64() uint64 { return uint64(b) }

var (
	byteSizePattern = regexp.MustCompile(`^\s*(\d+(?:\.\d+)?)\s*([KMGTP]?I?B?)\s*$`)

	byteSizeSuffix = map[string]uint64{
		"":    1,
		"B":   1,
		"K":   1 << 10,
		"KB":  1 << 10,
		"KIB": 1 << 10,
		"M":   1 << 20,
		"MB":  1 << 20,
		"MIB": 1 << 20,
		"G":   1 << 30,
		"GB":  1 << 30,
		"GIB": 1 << 30,
		"T":   1 << 40,
		"TB":  1 << 40,
		"TIB": 1 << 40,
		"P":   1 << 50,
		"PB":  1 << 50,
		"PIB": 1 << 50,
	}
)

func (b *ByteSize) UnmarshalText(text []byte) error {
	raw := strings.ToUpper(strings.TrimSpace(string(text)))
	if raw == "" {
		return errors.New("byte size is empty")
	}

	matches := byteSizePattern.FindStringSubmatch(raw)
	if matches == nil {
		return errors.Errorf("invalid byte size %q", string(text))
	}

	num, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return errors.Wrapf(err, "invalid byte size number %q", matches[1])
	}
	if num < 0 {
		return errors.Errorf("byte size must not be negative: %q", string(text))
	}

	multiplier, ok := byteSizeSuffix[matches[2]]
	if !ok {
		return errors.Errorf("unknown byte size suffix %q", matches[2])
	}

	*b = ByteSize(num * float64(multiplier))

	return nil
}
