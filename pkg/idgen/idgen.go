package idgen

import (
	"github.com/google/uuid"
	"github.com/rs/xid"
)

// New generates a globally unique, time-sortable, filesystem-safe ID.
func New() string {
	return xid.New().String()
}

// XIDToUUID converts a 12-byte XID into a 16-byte UUID v8 (RFC 9562).
// The 6 bits displaced by UUID version/variant markers are saved in byte 12,
// making the conversion lossless — UUIDToXID recovers the original XID.
func XIDToUUID(id xid.ID) uuid.UUID {
	var u uuid.UUID

	copy(u[0:6], id[0:6])
	u[6] = 0x80 | (id[6] & 0x0F)
	u[7] = id[7]
	u[8] = 0x80 | (id[8] & 0x3F)
	copy(u[9:12], id[9:12])
	u[12] = (id[6] & 0xF0) | ((id[8] & 0xC0) >> 4)

	return u
}

// UUIDToXID extracts the original 12-byte XID from a UUID produced by XIDToUUID.
// Calling this on a UUID not created by XIDToUUID yields an undefined result.
func UUIDToXID(u uuid.UUID) xid.ID {
	var id xid.ID

	copy(id[0:6], u[0:6])
	id[6] = (u[12] & 0xF0) | (u[6] & 0x0F)
	id[7] = u[7]
	id[8] = ((u[12] << 4) & 0xC0) | (u[8] & 0x3F)
	copy(id[9:12], u[9:12])

	return id
}
