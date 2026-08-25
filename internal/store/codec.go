package store

import (
	"encoding/json"
	"time"
)

func marshal(value any) (string, error) {
	data, err := json.Marshal(value)
	return string(data), err
}

func unmarshal(value string, target any) error { return json.Unmarshal([]byte(value), target) }

func timestamp(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }

func parseTime(value string) (time.Time, error) { return time.Parse(time.RFC3339Nano, value) }

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return timestamp(*value)
}

func optional(value string) any {
	if value == "" {
		return nil
	}
	return value
}
