package plugin

import (
	"encoding/base32"
	"encoding/binary"
	"hash/fnv"
	"strconv"
)

var IDEncoding = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz234567").WithPadding(base32.NoPadding)

func ParsePluginID(idStr string) uint {
	if id, err := strconv.ParseUint(idStr, 10, 64); err == nil {
		return uint(id)
	}

	if decoded, err := IDEncoding.DecodeString(idStr); err == nil && len(decoded) <= 8 {
		buf := make([]byte, 8)
		copy(buf[8-len(decoded):], decoded)

		return uint(binary.BigEndian.Uint64(buf))
	}

	h := fnv.New64a()
	_, _ = h.Write([]byte(idStr))

	return uint(h.Sum64())
}

func CompactPluginID(id uint) string {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(id))

	i := 0
	for i < len(buf)-1 && buf[i] == 0 {
		i++
	}

	return IDEncoding.EncodeToString(buf[i:])
}
