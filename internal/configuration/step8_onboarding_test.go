package configuration

import "testing"

func TestPolicySSHConsentBindsOnlyEligibleGrant(t *testing.T) {
	grant := FallbackGrant{RequestID: "request", Generation: 2, Reason: ReasonDaemonUnavailable}
	request, ok := PolicySSHConsent{}.From(grant, "request", 2)
	if !ok || !request.SSHConsent || request.FallbackClass != ReasonDaemonUnavailable {
		t.Fatalf("From() = %#v, %v", request, ok)
	}
	for _, denied := range []FallbackGrant{{RequestID: "request", Generation: 2, Reason: ReasonIdentityFailure}, {RequestID: "other", Generation: 2, Reason: ReasonDaemonUnavailable}, {RequestID: "request", Generation: 3, Reason: ReasonDaemonUnavailable}} {
		if _, ok := (PolicySSHConsent{}).From(denied, "request", 2); ok {
			t.Fatalf("grant was incorrectly accepted: %#v", denied)
		}
	}
}
