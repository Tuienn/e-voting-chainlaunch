package main

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func requireNonEmpty(value string, name string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", name)
	}
	return nil
}

func parseUint(value string, name string) (uint64, error) {
	if err := requireNonEmpty(value, name); err != nil {
		return 0, err
	}
	n, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be uint64: %w", name, err)
	}
	return n, nil
}

func parseHash32(value string, name string) ([]byte, error) {
	if err := requireNonEmpty(value, name); err != nil {
		return nil, err
	}
	v := strings.TrimSpace(value)
	if len(v) == 64 {
		if decoded, err := hex.DecodeString(v); err == nil && len(decoded) == 32 {
			return decoded, nil
		}
	}
	if decoded, err := base64.RawURLEncoding.DecodeString(v); err == nil && len(decoded) == 32 {
		return decoded, nil
	}
	if decoded, err := base64.URLEncoding.DecodeString(v); err == nil && len(decoded) == 32 {
		return decoded, nil
	}
	return nil, fmt.Errorf("%s must be 32 bytes encoded as hex64 or base64url", name)
}

func hashToBase64URL(hash []byte) string {
	return base64.RawURLEncoding.EncodeToString(hash)
}

func mustJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func mustJSONBytes(value any) []byte {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}
