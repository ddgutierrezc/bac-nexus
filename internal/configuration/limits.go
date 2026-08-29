package configuration

import "time"

// Release limits are Nexus policy, not profile inputs. Changing them is a
// versioned product decision because they bound remote resource usage.
const (
	MapepireServerVersion             = "2.3.5"
	DefaultMapepireDaemonPort         = 8076
	MaxMapepireFrameBytes             = 1 << 20
	MaxMapepireAggregateResponseBytes = 1 << 20
	MaxMapepireRowsPerPage            = 200
	MaxMapepireColumns                = 256
	MaxMapepireCursors                = 8
	MaxMapepirePendingRequests        = 64
)

const (
	MapepireHandshakeTimeout = 5 * time.Second
	MapepireRequestTimeout   = 15 * time.Second
	MapepireSessionLifetime  = 60 * time.Second
)
