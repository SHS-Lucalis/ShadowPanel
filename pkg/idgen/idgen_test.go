package idgen

import (
	"fmt"
	"testing"

	"github.com/gameap/gameap/pkg/base62"
	"github.com/google/uuid"
	"github.com/rs/xid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestXIDToUUID_and_UUIDToXID(t *testing.T) {
	tests := []struct {
		name string
		xid  xid.ID
	}{
		{
			name: "zero_xid",
			xid:  xid.NilID(),
		},
		{
			name: "all_bits_set",
			xid:  xid.ID{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
		},
		{
			name: "generated_xid",
			xid:  xid.New(),
		},
		{
			name: "high_nibble_byte6_set",
			xid:  xid.ID{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xF0, 0x00, 0x00, 0x00, 0x00, 0x00},
		},
		{
			name: "high_bits_byte8_set",
			xid:  xid.ID{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xC0, 0x00, 0x00, 0x00},
		},
		{
			name: "alternating_bits",
			xid:  xid.ID{0xAA, 0x55, 0xAA, 0x55, 0xAA, 0x55, 0xAA, 0x55, 0xAA, 0x55, 0xAA, 0x55},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u := XIDToUUID(tc.xid)

			assert.Equal(t, uuid.RFC4122, u.Variant(), "UUID variant must be RFC4122")
			assert.Equal(t, byte(8), byte(u.Version()), "UUID version must be 8")

			got := UUIDToXID(u)
			require.Equal(t, tc.xid, got, "round-trip must recover original XID")
		})
	}
}

func TestXIDToUUID_deterministic(t *testing.T) {
	id := xid.ID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0xAB, 0x08, 0xCD, 0x0A, 0x0B, 0x0C}

	u1 := XIDToUUID(id)
	u2 := XIDToUUID(id)

	assert.Equal(t, u1, u2, "same XID must produce same UUID")
}

func TestXIDToUUID_distinct(t *testing.T) {
	id1 := xid.New()
	id2 := xid.New()

	u1 := XIDToUUID(id1)
	u2 := XIDToUUID(id2)

	assert.NotEqual(t, u1, u2, "different XIDs must produce different UUIDs")
}

func TestXID(t *testing.T) {
	id := xid.New()

	fmt.Println("Generated XID:", id.String())
	fmt.Println("Generated XID (base62):", base62.EncodeToString(id.Bytes()))
	fmt.Println("Generated UUID:", XIDToUUID(id).String())

	u1 := XIDToUUID(id)
	fmt.Println("Generated UUID (base62):", base62.EncodeToString(u1[:]))

	u := uuid.New()
	fmt.Println("Generated UUID (base62):", base62.EncodeToString(u[:]))
}
