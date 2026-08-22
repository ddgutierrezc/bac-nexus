package opencode

import (
	"errors"

	"bac-nexus/internal/integrationpreview"
)

const Version = "v1"

type Request struct {
	Profile string
	Version string
}

type Preview struct {
	Client  integrationpreview.Client
	Version string
	Payload string
}

type serverConfig struct {
	Type    string   `json:"type"`
	Command []string `json:"command"`
}

type payload struct {
	MCP struct {
		Nexus serverConfig `json:"bac-nexus"`
	} `json:"mcp"`
}

func Build(request Request) (Preview, error) {
	if request.Version != "" && request.Version != Version {
		return Preview{}, errors.New("unsupported OpenCode preview version")
	}
	if err := integrationpreview.ValidateRequest(integrationpreview.Request{Profile: request.Profile}); err != nil {
		return Preview{}, err
	}
	value := payload{}
	value.MCP.Nexus = serverConfig{Type: "local", Command: []string{"nexus", "serve", "-profile", request.Profile}}
	encoded, err := integrationpreview.JSONPayload(value)
	if err != nil {
		return Preview{}, err
	}
	return Preview{Client: integrationpreview.ClientOpenCode, Version: Version, Payload: encoded}, nil
}
