package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

func decode(data []byte, out *fileConfig) error {
	// Reject null and duplicate fields as well as misspelled keys. Silently
	// accepting either can conceal an operator's intended configuration.
	dec := json.NewDecoder(bytes.NewReader(data))
	start, err := dec.Token()
	if err != nil || start != json.Delim('{') {
		return errors.New("configuration must be a JSON object")
	}
	seen := make(map[string]bool)
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return errors.New("invalid configuration JSON")
		}
		name, ok := key.(string)
		if !ok || seen[name] {
			return errors.New("configuration contains a duplicate field")
		}
		seen[name] = true
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return errors.New("configuration fields must have non-null values")
		}
	}
	if _, err := dec.Token(); err != nil {
		return errors.New("invalid configuration JSON")
	}
	if _, err := dec.Token(); err != io.EOF {
		return errors.New("configuration must contain exactly one JSON object")
	}
	dec = json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return errors.New("configuration contains unknown fields or invalid types")
	}
	return nil
}
