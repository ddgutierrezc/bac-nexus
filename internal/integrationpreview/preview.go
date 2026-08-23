package integrationpreview

import (
	"encoding/json"
	"errors"

	"bac-nexus/internal/profile"
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

func ValidateRequest(request Request) error {
	if profile.ValidateName(request.Profile) != nil {
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
