//go:build !windows

package codefori

// NewUnsupportedPlatformClient is the deterministic non-Windows constructor.
func NewUnsupportedPlatformClient() *Client {
	return NewClient(nil)
}

// NewPlatformClient keeps Companion unavailable where guarded Windows descriptor
// validation is not available.
func NewPlatformClient() *Client {
	return NewUnsupportedPlatformClient()
}
