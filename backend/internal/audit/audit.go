// Package audit records admin actions for the audit trail.
package audit

import (
	"context"
	"encoding/json"
	"net"

	"cashx/internal/repository"
)

// Recorder writes audit_log rows.
type Recorder struct {
	Q *repository.Queries
}

// Record appends one audit entry. changes is marshaled to JSON (nil allowed).
// actorID may be nil for system actions.
func (r *Recorder) Record(ctx context.Context, actorID *string, action, entityType, entityID string, changes any, ip net.IP) error {
	var raw []byte
	var err error
	if changes != nil {
		raw, err = json.Marshal(changes)
		if err != nil {
			return err
		}
	}
	var ipStr string
	if ip != nil {
		ipStr = ip.String()
	}
	return r.Q.InsertAuditLog(ctx, repository.InsertAuditLogParams{
		ActorUserID: repository.UUIDPtr(actorID),
		Action:      action,
		EntityType:  entityType,
		EntityID:    entityID,
		Changes:     raw,
		Column6:     ipStr,
	})
}
