package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
)

// JSON 是一个 GORM 友好的 JSON 列类型
type JSON []byte

func (j JSON) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return string(j), nil
}

func (j *JSON) Scan(v interface{}) error {
	if v == nil {
		*j = nil
		return nil
	}
	switch s := v.(type) {
	case []byte:
		*j = append((*j)[0:0], s...)
		return nil
	case string:
		*j = []byte(s)
		return nil
	}
	return errors.New("invalid json scan source")
}

func (j JSON) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("null"), nil
	}
	return j, nil
}

func (j *JSON) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		*j = nil
		return nil
	}
	*j = append((*j)[0:0], data...)
	return nil
}

// AsMap 解码 JSON 列到 map
func (j JSON) AsMap() (map[string]any, error) {
	if len(j) == 0 {
		return nil, nil
	}
	m := make(map[string]any)
	if err := json.Unmarshal(j, &m); err != nil {
		return nil, err
	}
	return m, nil
}
