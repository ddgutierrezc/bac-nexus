package integrationpreview

import (
	"encoding/json"
	"errors"
	"regexp"
)

type Client string

const (
	ClientCopilot  Client = "copilot"
	ClientOpenCode Client = "opencode"
)

type Request struct {
	Profile string
}

type Preview struct {
	Client  Client
	Version string
	Payload string
}

var profilePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

func ValidateRequest(request Request) error {
	if !profilePattern.MatchString(request.Profile) {
		return errors.New("integration preview requires a valid profile")
	}
	return nil
}

func JSONPayload(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", errors.New("integration preview serialization failed")
	}
	return string(data), nil
}
