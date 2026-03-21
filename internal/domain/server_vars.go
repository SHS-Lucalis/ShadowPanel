package domain

import (
	"database/sql/driver"
	"encoding/json"
)

type ServerVars map[string]string

func (v *ServerVars) Scan(value any) error {
	if value == nil {
		*v = nil

		return nil
	}

	var bytes []byte
	switch val := value.(type) {
	case []byte:
		bytes = val
	case string:
		bytes = []byte(val)
	default:
		return nil
	}

	if len(bytes) == 0 {
		*v = nil

		return nil
	}

	return json.Unmarshal(bytes, v)
}

func (v ServerVars) Value() (driver.Value, error) {
	if v == nil {
		return nil, nil
	}

	return json.Marshal(v)
}

func (v ServerVars) StringPtr() *string {
	if v == nil {
		return nil
	}

	bytes, err := json.Marshal(v)
	if err != nil {
		return nil
	}

	s := string(bytes)

	return &s
}
