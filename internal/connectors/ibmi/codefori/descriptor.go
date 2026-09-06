package codefori

// DescriptorReader is injected at composition time so this connector never owns
// profile, credential, or platform-specific descriptor access.
type DescriptorReader func() (Descriptor, error)

// Descriptor carries the local capability required by the fixed Companion RPC.
type Descriptor struct {
	Version    int
	Generation string
	Token      string
}

func (descriptor Descriptor) valid() bool {
	return descriptor.Version == protocolVersion && descriptor.Generation != "" && descriptor.Token != ""
}
