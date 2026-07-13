package httpapi_test

import (
	"net/http"
	"testing"

	"git.bitcrusher32.win/bitcrusher32/carbonstack-cypher/internal/db"
)

func TestRelaySpaceMemberStateHTTPControlsRoutingAndPreservesRejoinBoundary(
	t *testing.T,
) {
	server := newTestServer(t)
	defer server.Close()

	alice := claimInvite(t, server.URL, "dev-invite", "alice-member-state")
	createDevInvite(t, server.URL, "bob-member-state-invite")
	bob := claimInvite(
		t,
		server.URL,
		"bob-member-state-invite",
		"bob-member-state",
	)

	aliceDevice := registerDevice(
		t,
		server.URL,
		alice.AccountID,
		"alice-member-state-device",
		"alice-member-state-public-key",
		"alice-member-state-prekey",
	)
	bobDevice := registerDevice(
		t,
		server.URL,
		bob.AccountID,
		"bob-member-state-device",
		"bob-member-state-public-key",
		"bob-member-state-prekey",
	)

	space := createRelaySpace(t, server.URL, map[string]any{
		"relay_space_id":        "member-state-http-space",
		"display_label":         "member state HTTP space",
		"created_by_account_id": alice.AccountID,
		"created_by_device_id":  aliceDevice.DeviceID,
	})
	aliceMember := registerRelaySpaceMember(
		t,
		server.URL,
		space.RelaySpaceID,
		map[string]any{
			"routing_member_id": "alice-member-state-routing",
			"account_id":        alice.AccountID,
			"device_id":         aliceDevice.DeviceID,
		},
	)
	bobMember := registerRelaySpaceMember(
		t,
		server.URL,
		space.RelaySpaceID,
		map[string]any{
			"routing_member_id": "bob-member-state-routing",
			"account_id":        bob.AccountID,
			"device_id":         bobDevice.DeviceID,
		},
	)

	stateURL := server.URL +
		"/v0/relay-spaces/" +
		space.RelaySpaceID +
		"/members/" +
		bobMember.RoutingMemberID +
		"/state"

	var disabled db.RelaySpaceMemberStateResult
	doPost(
		t,
		stateURL,
		map[string]any{"target_state": "disabled"},
		http.StatusOK,
		&disabled,
	)
	if disabled.CurrentState != db.RelaySpaceMemberStateDisabled ||
		disabled.Idempotent ||
		disabled.RoutingMember.DisabledAt == "" {
		t.Fatalf("unexpected disabled result: %+v", disabled)
	}

	var disabledAgain db.RelaySpaceMemberStateResult
	doPost(
		t,
		stateURL,
		map[string]any{"target_state": "disabled"},
		http.StatusOK,
		&disabledAgain,
	)
	if !disabledAgain.Idempotent ||
		disabledAgain.TransitionClassification !=
			db.RelaySpaceMemberStateAlreadyCurrent {
		t.Fatalf("unexpected idempotent disable: %+v", disabledAgain)
	}

	var recipientRefusal errorResponse
	doPost(
		t,
		server.URL+"/v0/relay-spaces/"+
			space.RelaySpaceID+
			"/envelopes",
		map[string]any{
			"sender_device_id":    aliceDevice.DeviceID,
			"recipient_device_id": bobDevice.DeviceID,
			"content_type":        "carbonstack.message.text.stub.v0",
			"protocol_version":    "stub-v0",
			"ciphertext_b64":      "AQID",
		},
		http.StatusForbidden,
		&recipientRefusal,
	)
	if recipientRefusal.Error.Code != "recipient_not_relay_member" {
		t.Fatalf(
			"error code = %q, want recipient_not_relay_member",
			recipientRefusal.Error.Code,
		)
	}

	var reactivated db.RelaySpaceMemberStateResult
	doPost(
		t,
		stateURL,
		map[string]any{"target_state": "active"},
		http.StatusOK,
		&reactivated,
	)
	if reactivated.CurrentState != db.RelaySpaceMemberStateActive ||
		reactivated.RoutingMember.DisabledAt != "" {
		t.Fatalf("unexpected reactivation result: %+v", reactivated)
	}

	var submitted map[string]any
	doPost(
		t,
		server.URL+"/v0/relay-spaces/"+
			space.RelaySpaceID+
			"/envelopes",
		map[string]any{
			"sender_device_id":    aliceDevice.DeviceID,
			"recipient_device_id": bobDevice.DeviceID,
			"content_type":        "carbonstack.message.text.stub.v0",
			"protocol_version":    "stub-v0",
			"ciphertext_b64":      "AQID",
		},
		http.StatusCreated,
		&submitted,
	)

	var left db.RelaySpaceMemberStateResult
	doPost(
		t,
		stateURL,
		map[string]any{"target_state": "left"},
		http.StatusOK,
		&left,
	)
	if left.CurrentState != db.RelaySpaceMemberStateLeft ||
		left.RoutingMember.DisabledAt != "" {
		t.Fatalf("unexpected left result: %+v", left)
	}

	var rejoinRequired errorResponse
	doPost(
		t,
		stateURL,
		map[string]any{"target_state": "active"},
		http.StatusConflict,
		&rejoinRequired,
	)
	if rejoinRequired.Error.Code !=
		"relay_space_member_rejoin_required" {
		t.Fatalf(
			"error code = %q, want relay_space_member_rejoin_required",
			rejoinRequired.Error.Code,
		)
	}

	var senderRefusal errorResponse
	aliceStateURL := server.URL +
		"/v0/relay-spaces/" +
		space.RelaySpaceID +
		"/members/" +
		aliceMember.RoutingMemberID +
		"/state"
	doPost(
		t,
		aliceStateURL,
		map[string]any{"target_state": "disabled"},
		http.StatusOK,
		&db.RelaySpaceMemberStateResult{},
	)
	doPost(
		t,
		server.URL+"/v0/relay-spaces/"+
			space.RelaySpaceID+
			"/envelopes",
		map[string]any{
			"sender_device_id":    aliceDevice.DeviceID,
			"recipient_device_id": bobDevice.DeviceID,
			"content_type":        "carbonstack.message.text.stub.v0",
			"protocol_version":    "stub-v0",
			"ciphertext_b64":      "AQID",
		},
		http.StatusForbidden,
		&senderRefusal,
	)
	if senderRefusal.Error.Code != "sender_not_relay_member" {
		t.Fatalf(
			"error code = %q, want sender_not_relay_member",
			senderRefusal.Error.Code,
		)
	}
}

func TestRelaySpaceMemberStateHTTPRefusesWrongSpaceAndUnsupportedTarget(
	t *testing.T,
) {
	server := newTestServer(t)
	defer server.Close()

	alice := claimInvite(t, server.URL, "dev-invite", "alice-state-refusal")
	aliceDevice := registerDevice(
		t,
		server.URL,
		alice.AccountID,
		"alice-state-refusal-device",
		"alice-state-refusal-public-key",
		"alice-state-refusal-prekey",
	)

	space := createRelaySpace(t, server.URL, map[string]any{
		"relay_space_id":        "state-refusal-space",
		"display_label":         "state refusal space",
		"created_by_account_id": alice.AccountID,
		"created_by_device_id":  aliceDevice.DeviceID,
	})
	member := registerRelaySpaceMember(
		t,
		server.URL,
		space.RelaySpaceID,
		map[string]any{
			"routing_member_id": "state-refusal-member",
			"account_id":        alice.AccountID,
			"device_id":         aliceDevice.DeviceID,
		},
	)

	var wrongSpace errorResponse
	doPost(
		t,
		server.URL+
			"/v0/relay-spaces/other-space/members/"+
			member.RoutingMemberID+
			"/state",
		map[string]any{"target_state": "disabled"},
		http.StatusConflict,
		&wrongSpace,
	)
	if wrongSpace.Error.Code != "relay_space_member_wrong_space" {
		t.Fatalf(
			"error code = %q, want relay_space_member_wrong_space",
			wrongSpace.Error.Code,
		)
	}

	var unsupported errorResponse
	doPost(
		t,
		server.URL+
			"/v0/relay-spaces/"+
			space.RelaySpaceID+
			"/members/"+
			member.RoutingMemberID+
			"/state",
		map[string]any{"target_state": "removed"},
		http.StatusBadRequest,
		&unsupported,
	)
	if unsupported.Error.Code !=
		"relay_space_member_target_state_unsupported" {
		t.Fatalf(
			"error code = %q",
			unsupported.Error.Code,
		)
	}
}
