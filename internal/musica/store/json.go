package store

import (
	"encoding/json"
	"strings"
)

func trimJSON(raw string) string {
	return strings.TrimSpace(raw)
}

func unmarshalSetting(raw string, dest any) error {
	return json.Unmarshal([]byte(raw), dest)
}
