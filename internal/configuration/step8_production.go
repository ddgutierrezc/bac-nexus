package configuration

import "time"

// Step8ProductionDependencies names the already-owned application boundaries
// required to compose one production Step 8 runner. It does not expose a
// generic SQL, shell, download, retry, or alternate-transport surface.
type Step8ProductionDependencies struct {
	Observe        Step8PreAuth
	Credentials    CredentialProvider
	WSS            Step8WSSFactory
	SSHPolicy      SSHPolicy
	SSHTrust       SSHTrust
	SSHCredentials CredentialProvider
	SSH            Step8SSHRuntimeFactory
	Markers        Step8MarkerStore
	Audit          Step8Auditor
	NowUnixMs      func() int64
}

// NewStep8Production preserves the service-owned WSS-first ordering. Only the
// typed eligible observations can reach the existing policy/trust/consent/
// credential gate and fixed SSH runtime; all other classifications stay
// terminal in Step8Service.
func NewStep8Production(deps Step8ProductionDependencies) Step8Service {
	return Step8Service{
		Observe:     deps.Observe,
		Credentials: deps.Credentials,
		WSS:         deps.WSS,
		Gate: PostObservationGate{
			Policy:      deps.SSHPolicy,
			Trust:       deps.SSHTrust,
			Credentials: deps.SSHCredentials,
		},
		SSH:       deps.SSH,
		Markers:   deps.Markers,
		Audit:     deps.Audit,
		NowUnixMs: deps.NowUnixMs,
		Tickets:   newFallbackTicketStore(time.Now),
	}
}
