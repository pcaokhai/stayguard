package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/pcaokhai/stayguard/api/internal/domain/payment"
	"github.com/pcaokhai/stayguard/api/internal/domain/stay"
)

const (
	routeDismissFmt     = "POST /v1/owner/payment-events/%s/dismiss"
	auditPayDismissed   = "payment.dismissed"
	maxDismissNoteRunes = 500
)

// DismissedTransfer is the answer of dismissUnmatchedTransfer (and its stored idempotency body).
type DismissedTransfer struct {
	EventID string `json:"eventId"`
	Result  string `json:"result"`
	Note    string `json:"note"`
}

// DismissTransfer lets the owner close an unmatched inbound transfer that belongs to no bill (money sent by mistake, a refund, a
// foreign payment) with a note saying why. The event is kept as DISMISSED for audit, its UNMATCHED_TRANSFER alert is resolved, nothing
// is settled, and it is audited as payment.dismissed. Only an UNMATCHED event can be dismissed; a repeat with the same key returns the
// same answer, with another key it is a 409.
func (p *Payments) DismissTransfer(ctx context.Context, c Caller, eventID, retryID, note string) (DismissedTransfer, error) {
	const op = "dismissUnmatchedTransfer"
	if err := p.checkRole(op, c); err != nil {
		return DismissedTransfer{}, err
	}
	if err := checkKey(retryID); err != nil {
		return DismissedTransfer{}, err
	}
	note = strings.TrimSpace(note)
	switch {
	case note == "":
		return DismissedTransfer{}, stay.NewValidationError([]stay.FieldError{{Path: "note", Code: stay.CodeRequired}})
	case utf8.RuneCountInString(note) > maxDismissNoteRunes:
		return DismissedTransfer{}, stay.NewValidationError([]stay.FieldError{{Path: "note", Code: stay.CodeTooLong}})
	}
	var out DismissedTransfer
	err := p.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		stored, ok, err := p.repo.LockEvent(ctx, tx, eventID)
		if err != nil {
			return fmt.Errorf("lock event: %w", err)
		}
		if !ok {
			return ErrNotFound
		}
		route := fmt.Sprintf(routeDismissFmt, eventID)
		oc, err := p.idem.Begin(ctx, tx, route, retryID, RequestHash([]byte(`{"note":`+quoteJSON(note)+`}`)))
		if err != nil {
			return err
		}
		if oc.Replay {
			return json.Unmarshal(oc.Body, &out)
		}
		if stored.Result != payment.ResultUnmatched {
			return ErrEventNotDismissable
		}
		now := storedTime(p.clock.Now())
		done, err := p.repo.DismissEvent(ctx, tx, stored.ID, note, c.UserID, now)
		if err != nil {
			return err
		}
		if !done {
			return ErrEventNotDismissable
		}
		if err := p.resolveAlerts(ctx, tx, AlertResolve{EventID: stored.ID, EventNote: clip(stored.Event.Content), EventAmount: stored.Event.Amount, Resolution: ResolutionDismissed}); err != nil {
			return err
		}
		after, err := json.Marshal(map[string]any{"eventId": stored.ID, "amount": stored.Event.Amount, "note": note})
		if err != nil {
			return fmt.Errorf("encode audit: %w", err)
		}
		e := AuditEntry{ID: p.ids.New(auditIDPrefix), ActorID: c.UserID, Action: auditPayDismissed, EntityType: "payment_event", EntityID: stored.ID, After: after}
		if err := p.audit.Append(ctx, tx, e); err != nil {
			return fmt.Errorf("audit: %w", err)
		}
		out = DismissedTransfer{EventID: stored.ID, Result: payment.ResultDismissed, Note: note}
		body, err := json.Marshal(out)
		if err != nil {
			return fmt.Errorf("encode response: %w", err)
		}
		return p.idem.Complete(ctx, tx, route, retryID, statusOK, body)
	})
	return out, err
}
