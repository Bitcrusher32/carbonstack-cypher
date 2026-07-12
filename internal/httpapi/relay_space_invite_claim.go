package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"git.bitcrusher32.win/bitcrusher32/carbonstack-cypher/internal/db"
)

type claimRelaySpaceInviteRequest struct {
	InviteToken  string `json:"invite_token"`
	AccountID    string `json:"account_id"`
	DeviceID     string `json:"device_id"`
	DisplayLabel string `json:"display_label"`
}

func (a *API) claimRelaySpaceInvite(w http.ResponseWriter, r *http.Request) {
	var req claimRelaySpaceInviteRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	req.InviteToken = strings.TrimSpace(req.InviteToken)
	req.AccountID = strings.TrimSpace(req.AccountID)
	req.DeviceID = strings.TrimSpace(req.DeviceID)
	req.DisplayLabel = strings.TrimSpace(req.DisplayLabel)

	if req.InviteToken == "" ||
		req.AccountID == "" ||
		req.DeviceID == "" {
		writeError(
			w,
			http.StatusBadRequest,
			"invalid_request",
			"invite_token, account_id, and device_id are required",
		)
		return
	}

	result, err := a.store.ClaimRelaySpaceInvite(db.ClaimRelaySpaceInviteInput{
		InviteToken:  req.InviteToken,
		AccountID:    req.AccountID,
		DeviceID:     req.DeviceID,
		DisplayLabel: req.DisplayLabel,
	})
	if err != nil {
		if writeRelaySpaceIntegrityError(w, err) {
			return
		}

		switch {
		case errors.Is(err, db.ErrRelaySpaceInviteNotFound):
			writeError(w, http.StatusNotFound, "relay_space_invite_invalid", "relay space invite is invalid")
		case errors.Is(err, db.ErrRelaySpaceInviteExpired):
			writeError(w, http.StatusGone, "relay_space_invite_expired", "relay space invite is expired")
		case errors.Is(err, db.ErrRelaySpaceInviteDisabled):
			writeError(w, http.StatusConflict, "relay_space_invite_disabled", "relay space invite is disabled")
		case errors.Is(err, db.ErrRelaySpaceInviteExhausted):
			writeError(w, http.StatusConflict, "relay_space_invite_exhausted", "relay space invite is exhausted")
		case errors.Is(err, db.ErrRelaySpaceMemberAccountConflict):
			writeError(w, http.StatusConflict, "relay_space_member_account_conflict", "device already belongs to another account in this relay space")
		case errors.Is(err, db.ErrRelaySpaceMemberDisabled):
			writeError(w, http.StatusConflict, "relay_space_member_disabled", "routing member is disabled and requires explicit operator action")
		case errors.Is(err, db.ErrRelaySpaceMemberLeft):
			writeError(w, http.StatusConflict, "relay_space_member_left", "routing member has left and requires explicit rejoin action")
		case errors.Is(err, db.ErrRelaySpaceInviteUnsupportedState):
			writeError(w, http.StatusConflict, "relay_space_invite_state_unsupported", "relay space invite state is unsupported")
		case errors.Is(err, db.ErrRelaySpaceInviteClaimContended):
			writeError(w, http.StatusConflict, "relay_space_invite_claim_contended", "relay space invite claim remained contended; retry inspection")
		case errors.Is(err, db.ErrRelaySpaceInviteExpiryInvalid):
			writeError(w, http.StatusInternalServerError, "relay_space_invite_expiry_invalid", "stored relay space invite expiry is invalid")
		default:
			writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		}
		return
	}

	status := http.StatusCreated
	if result.Idempotent {
		status = http.StatusOK
	}
	writeJSON(w, status, result)
}
