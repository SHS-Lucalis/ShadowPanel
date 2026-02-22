package domain

import (
	"database/sql/driver"
	"encoding/json"
)

type Metadata map[string]any

func (m *Metadata) Scan(value any) error {
	if value == nil {
		*m = nil

		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return nil
	}

	if len(bytes) == 0 {
		*m = nil

		return nil
	}

	return json.Unmarshal(bytes, m)
}

func (m Metadata) Value() (driver.Value, error) {
	if m == nil {
		return nil, nil
	}

	return json.Marshal(m)
}

// String returns the JSON representation of the metadata.
func (m Metadata) String() string {
	if m == nil {
		return ""
	}
	bytes, err := json.Marshal(m)
	if err != nil {
		return ""
	}

	return string(bytes)
}
