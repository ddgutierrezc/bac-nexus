package configuration

import (
	"context"
	"testing"

	"bac-nexus/internal/profile"
)

func TestNewStep8ProductionUsesWSSWithoutFallbackRuntime(t *testing.T) {
	secret := []byte("opaque")
	ssh := &serviceSSHFactory{}
	trace := []string{}
	service := NewStep8Production(Step8ProductionDependencies{
		Observe: step8ObserveFunc(func(context.Context, profile.Profile) Observation {
			return Observation{Decision: DecisionWSSSelected, Reason: ReasonWSSSelected}
		}),
		Credentials: step8CredentialsFunc(func(context.Context, string, profile.CredentialMode) ([]byte, error) {
			return secret, nil
		}),
		WSS: step8WSSFunc(func(context.Context, profile.Profile) (Step8WSSSession, error) {
			return &step8Session{trace: &trace}, nil
		}),
		SSH: ssh,
	})

	result := service.Run(context.Background(), Step8Request{RequestID: "wss", Generation: 1, Profile: serviceSavedProfile(), WSSConsent: true})
	if result.Decision != DecisionWSSSelected || result.Class != ResultProofSuccess || !result.Cleanup {
		t.Fatalf("result=%+v", result)
	}
	if ssh.calls != 0 {
		t.Fatalf("SSH runtime calls=%d, want 0", ssh.calls)
	}
	for _, b := range secret {
		if b != 0 {
			t.Fatal("credential was not zeroed after WSS cleanup")
		}
	}
}

func TestNewStep8ProductionRoutesOnlyEligibleObservationsToGatedSSH(t *testing.T) {
	eligible := []Step8Reason{ReasonDaemonRefused, ReasonDaemonUnavailable, ReasonDaemonAvailabilityTimeout, ReasonDaemonPolicyDisabled, ReasonUnsupportedVersion}
	for _, reason := range eligible {
		t.Run(string(reason), func(t *testing.T) {
			gate := &gateFake{credential: []byte("opaque")}
			client := &serviceSSHClient{}
			ssh := &serviceSSHFactory{runtime: &SSHRuntime{client: client}}
			service := NewStep8Production(Step8ProductionDependencies{
				Observe: step8ObserveFunc(func(context.Context, profile.Profile) Observation {
					return Observation{Decision: DecisionSSHEligible, Reason: reason}
				}),
				SSHPolicy: gate, SSHTrust: gate, SSHCredentials: gate, SSH: ssh,
			})

			wss := service.Run(context.Background(), Step8Request{RequestID: "ssh", Generation: 1, Profile: serviceSavedProfile(), WSSConsent: true})
			if wss.FallbackTicket == "" || ssh.calls != 0 {
				t.Fatalf("wss=%+v SSH=%d", wss, ssh.calls)
			}
			result := service.RunSSH(context.Background(), Step8Request{RequestID: "ssh", Generation: 1, Profile: serviceSavedProfile(), FallbackTicket: wss.FallbackTicket, FallbackClass: wss.FallbackClass, SSHConsent: true})
			if result.Decision != DecisionSSHEligible || result.Class != ResultProofSuccess || !result.Cleanup {
				t.Fatalf("result=%+v", result)
			}
			if got, want := joinTrace(gate.calls), "policy,trust,credential"; got != want || ssh.calls != 1 || client.proofs != 1 || client.closes != 1 {
				t.Fatalf("gate=%q SSH=%d proof=%d close=%d", got, ssh.calls, client.proofs, client.closes)
			}
		})
	}
}

func TestNewStep8ProductionFailsClosedForTerminalAndUnknownObservations(t *testing.T) {
	for _, observation := range []Observation{
		{Decision: DecisionTerminal, Reason: ReasonIdentityFailure},
		{Decision: DecisionTerminal, Reason: ReasonProtocolFailure},
		{Decision: DecisionTerminal, Reason: ReasonMalformedResponse},
		{Decision: DecisionTerminal, Reason: ReasonDowngradeBlocked},
		{Decision: DecisionTerminal, Reason: ReasonCancelled},
		{Decision: DecisionTerminal, Reason: ReasonOperationTimeout},
		{Decision: DecisionTerminal, Reason: ReasonLimitExceeded},
		{Decision: DecisionTerminal, Reason: ReasonCredentialsUnavailable},
		{Decision: Decision("unknown"), Reason: Step8Reason("unknown")},
	} {
		t.Run(string(observation.Reason), func(t *testing.T) {
			gate := &gateFake{}
			ssh := &serviceSSHFactory{}
			service := NewStep8Production(Step8ProductionDependencies{
				Observe:   step8ObserveFunc(func(context.Context, profile.Profile) Observation { return observation }),
				SSHPolicy: gate, SSHTrust: gate, SSHCredentials: gate, SSH: ssh,
			})
			result := service.Run(context.Background(), Step8Request{RequestID: "terminal", Generation: 1, Profile: serviceSavedProfile(), WSSConsent: true})
			if result.Decision != DecisionTerminal || ssh.calls != 0 || len(gate.calls) != 0 {
				t.Fatalf("result=%+v SSH=%d gate=%v", result, ssh.calls, gate.calls)
			}
		})
	}
}
