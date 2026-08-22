package copilot

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
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

type payload struct {
	Servers struct {
		Nexus serverConfig `json:"bac-nexus"`
	} `json:"servers"`
}

func Build(request Request) (Preview, error) {
	if request.Version != "" && request.Version != Version {
		return Preview{}, errors.New("unsupported Copilot preview version")
	}
	if err := integrationpreview.ValidateRequest(integrationpreview.Request{Profile: request.Profile}); err != nil {
		return Preview{}, err
	}
	value := payload{}
	value.Servers.Nexus = serverConfig{Command: "nexus", Args: []string{"serve", "-profile", request.Profile}}
	encoded, err := integrationpreview.JSONPayload(value)
	if err != nil {
		return Preview{}, err
	}
	return Preview{Client: integrationpreview.ClientCopilot, Version: Version, Payload: encoded}, nil
}
