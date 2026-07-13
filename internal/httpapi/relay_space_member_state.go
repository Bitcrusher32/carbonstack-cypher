package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"git.bitcrusher32.win/bitcrusher32/carbonstack-cypher/internal/db"
)

type updateRelaySpaceMemberStateRequest struct {
	TargetState string `json:"target_state"`
}

func (a *API) updateRelaySpaceMemberState(
	w http.ResponseWriter,
	r *http.Request,
) {
	relaySpaceID := strings.TrimSpace(
		r.PathValue("relay_space_id"),
	)
	routingMemberID := strings.TrimSpace(
		r.PathValue("routing_member_id"),
	)
	if relaySpaceID == "" || routingMemberID == "" {
		writeError(
			w,
			http.StatusNotFound,
			"not_found",
			"route not found",
		)
		return
	}

	var req updateRelaySpaceMemberStateRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req.TargetState = strings.TrimSpace(req.TargetState)
	if req.TargetState == "" {
		writeError(
			w,
			http.StatusBadRequest,
			"invalid_request",
			"target_state is required",
		)
		return
	}

	result, err := a.store.UpdateRelaySpaceMemberState(
		db.UpdateRelaySpaceMemberStateInput{
			RelaySpaceID:    relaySpaceID,
			RoutingMemberID: routingMemberID,
			TargetState:     req.TargetState,
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, db.ErrRelaySpaceMemberNotFound):
			writeError(
				w,
				http.StatusNotFound,
				"relay_space_member_not_found",
				"relay space routing member not found",
			)
		case errors.Is(err, db.ErrRelaySpaceMemberWrongSpace):
			writeError(
				w,
				http.StatusConflict,
				"relay_space_member_wrong_space",
				"routing member belongs to another relay space",
			)
		case errors.Is(
			err,
			db.ErrRelaySpaceMemberTargetStateUnsupported,
		):
			writeError(
				w,
				http.StatusBadRequest,
				"relay_space_member_target_state_unsupported",
				"target_state must be active, disabled, or left",
			)
		case errors.Is(err, db.ErrRelaySpaceMemberRejoinRequired):
			writeError(
				w,
				http.StatusConflict,
				"relay_space_member_rejoin_required",
				"left routing member requires an explicit rejoin workflow",
			)
		case errors.Is(
			err,
			db.ErrRelaySpaceMemberStateInconsistent,
		):
			writeError(
				w,
				http.StatusInternalServerError,
				"relay_space_member_state_inconsistent",
				"stored routing-member state is inconsistent",
			)
		case errors.Is(
			err,
			db.ErrRelaySpaceMemberStoredStateUnsupported,
		):
			writeError(
				w,
				http.StatusInternalServerError,
				"relay_space_member_stored_state_unsupported",
				"stored routing-member state is unsupported",
			)
		case errors.Is(err, db.ErrRelaySpaceMemberStateContended):
			writeError(
				w,
				http.StatusConflict,
				"relay_space_member_state_contended",
				"routing-member state transition remained contended",
			)
		default:
			writeError(
				w,
				http.StatusInternalServerError,
				"db_error",
				err.Error(),
			)
		}
		return
	}

	writeJSON(w, http.StatusOK, result)
}
