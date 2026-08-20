package source

import (
	"context"
	"errors"
	"time"
)

var ErrOwnershipInvalid = errors.New("ownership record is invalid")
var ErrOwnershipMismatch = errors.New("ownership record does not match existing token")
var ErrOwnershipCapacity = errors.New("ownership ledger capacity exceeded")

type OwnershipRecord struct {
	Token        []byte
	RemotePath   string
	Profile      string
	TargetDigest []byte
	CreatedAt    time.Time
}
type OwnershipLedger interface {
	Admit(context.Context, OwnershipRecord) error
	Close() error
}
