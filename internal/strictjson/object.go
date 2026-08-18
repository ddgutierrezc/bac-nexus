package strictjson

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// ValidateObjectKeys rejects unknown, duplicate, and case-variant keys before
// callers decode a flat JSON object into a typed value.
func ValidateObjectKeys(data []byte, allowedKeys ...string) error {
	allowed := make(map[string]struct{}, len(allowedKeys))
	for _, key := range allowedKeys {
		allowed[key] = struct{}{}
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	first, err := decoder.Token()
	if err != nil {
		return err
	}
	if delimiter, ok := first.(json.Delim); !ok || delimiter != '{' {
		return errors.New("JSON value must be an object")
	}
	seen := make(map[string]string, len(allowedKeys))
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		key, ok := token.(string)
		if !ok {
			return errors.New("JSON object key is not a string")
		}
		folded := strings.ToLower(key)
		if previous, exists := seen[folded]; exists {
			return fmt.Errorf("duplicate or case-variant JSON key %q and %q", previous, key)
		}
		seen[folded] = key
		if _, ok := allowed[key]; !ok {
			return fmt.Errorf("unknown JSON key %q", key)
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return err
		}
	}
	last, err := decoder.Token()
	if err != nil {
		return err
	}
	if delimiter, ok := last.(json.Delim); !ok || delimiter != '}' {
		return errors.New("JSON object is not terminated")
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("trailing JSON value")
		}
		return err
	}
	return nil
}
