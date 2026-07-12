package httpapi_test

import (
	"net/http"
	"testing"

	"git.bitcrusher32.win/bitcrusher32/carbonstack-cypher/internal/db"
)

func TestRelaySpaceInviteClaimHTTPAtomicLifecycle(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	alice := claimInvite(t, server.URL, "dev-invite", "alice-claim-http")
	createDevInvite(t, server.URL, "bob-claim-http-invite")
	bob := claimInvite(t, server.URL, "bob-claim-http-invite", "bob-claim-http")
	createDevInvite(t, server.URL, "charlie-claim-http-invite")
	charlie := claimInvite(
		t,
		server.URL,
		"charlie-claim-http-invite",
		"charlie-claim-http",
	)

	aliceDevice := registerDevice(
		t,
		server.URL,
		alice.AccountID,
		"alice-claim-http-device",
		"alice-claim-http-public-key",
		"alice-claim-http-prekey",
	)
	bobDevice := registerDevice(
		t,
		server.URL,
		bob.AccountID,
		"bob-claim-http-device",
		"bob-claim-http-public-key",
		"bob-claim-http-prekey",
	)
	charlieDevice := registerDevice(
		t,
		server.URL,
		charlie.AccountID,
		"charlie-claim-http-device",
		"charlie-claim-http-public-key",
		"charlie-claim-http-prekey",
	)

	space := createRelaySpace(t, server.URL, map[string]any{
		"relay_space_id":        "claim-http-space",
		"display_label":         "claim HTTP space",
		"created_by_account_id": alice.AccountID,
		"created_by_device_id":  aliceDevice.DeviceID,
	})
	creator := registerRelaySpaceMember(
		t,
		server.URL,
		space.RelaySpaceID,
		map[string]any{
			"routing_member_id": "claim-http-creator",
			"account_id":        alice.AccountID,
			"device_id":         aliceDevice.DeviceID,
		},
	)

	maxClaims := 1
	invite := createRelaySpaceInvite(
		t,
		server.URL,
		space.RelaySpaceID,
		map[string]any{
			"relay_space_invite_id": "claim-http-invite",
			"invite_token":          "claim-http-token",
			"display_code":          "CLAIM-HTTP",
			"created_by_member_id":  creator.RoutingMemberID,
			"max_claims":            maxClaims,
		},
	)

	var created db.RelaySpaceInviteClaimResult
	doPost(
		t,
		server.URL+"/v0/relay-spaces/invites/claim",
		map[string]any{
			"invite_token":  invite.InviteToken,
			"account_id":    bob.AccountID,
			"device_id":     bobDevice.DeviceID,
			"display_label": "Bob",
		},
		http.StatusCreated,
		&created,
	)
	if created.ClaimClassification != db.RelaySpaceInviteClaimCreated ||
		created.Idempotent ||
		!created.ClaimConsumed ||
		created.RelaySpaceInvite.ClaimCount != 1 {
		t.Fatalf("unexpected created claim: %+v", created)
	}

	var retry db.RelaySpaceInviteClaimResult
	doPost(
		t,
		server.URL+"/v0/relay-spaces/invites/claim",
		map[string]any{
			"invite_token": invite.InviteToken,
			"account_id":   bob.AccountID,
			"device_id":    bobDevice.DeviceID,
		},
		http.StatusOK,
		&retry,
	)
	if !retry.Idempotent ||
		retry.ClaimConsumed ||
		retry.RelaySpaceInvite.ClaimCount != 1 {
		t.Fatalf("unexpected retry: %+v", retry)
	}

	var exhausted errorResponse
	doPost(
		t,
		server.URL+"/v0/relay-spaces/invites/claim",
		map[string]any{
			"invite_token": invite.InviteToken,
			"account_id":   charlie.AccountID,
			"device_id":    charlieDevice.DeviceID,
		},
		http.StatusConflict,
		&exhausted,
	)
	if exhausted.Error.Code != "relay_space_invite_exhausted" {
		t.Fatalf("error code = %q", exhausted.Error.Code)
	}

	var invalid errorResponse
	doPost(
		t,
		server.URL+"/v0/relay-spaces/invites/claim",
		map[string]any{
			"invite_token": "not-a-real-token",
			"account_id":   charlie.AccountID,
			"device_id":    charlieDevice.DeviceID,
		},
		http.StatusNotFound,
		&invalid,
	)
	if invalid.Error.Code != "relay_space_invite_invalid" {
		t.Fatalf("error code = %q", invalid.Error.Code)
	}
}

func TestRelaySpaceInviteClaimHTTPRefusesMismatchAndExpiryWithoutConsuming(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	alice := claimInvite(t, server.URL, "dev-invite", "alice-claim-refusal")
	createDevInvite(t, server.URL, "bob-claim-refusal-invite")
	bob := claimInvite(
		t,
		server.URL,
		"bob-claim-refusal-invite",
		"bob-claim-refusal",
	)
	createDevInvite(t, server.URL, "charlie-claim-refusal-invite")
	charlie := claimInvite(
		t,
		server.URL,
		"charlie-claim-refusal-invite",
		"charlie-claim-refusal",
	)

	aliceDevice := registerDevice(
		t,
		server.URL,
		alice.AccountID,
		"alice-claim-refusal-device",
		"alice-claim-refusal-public-key",
		"alice-claim-refusal-prekey",
	)
	bobDevice := registerDevice(
		t,
		server.URL,
		bob.AccountID,
		"bob-claim-refusal-device",
		"bob-claim-refusal-public-key",
		"bob-claim-refusal-prekey",
	)
	charlieDevice := registerDevice(
		t,
		server.URL,
		charlie.AccountID,
		"charlie-claim-refusal-device",
		"charlie-claim-refusal-public-key",
		"charlie-claim-refusal-prekey",
	)

	space := createRelaySpace(t, server.URL, map[string]any{
		"relay_space_id":        "claim-refusal-space",
		"display_label":         "claim refusal space",
		"created_by_account_id": alice.AccountID,
		"created_by_device_id":  aliceDevice.DeviceID,
	})
	creator := registerRelaySpaceMember(
		t,
		server.URL,
		space.RelaySpaceID,
		map[string]any{
			"routing_member_id": "claim-refusal-creator",
			"account_id":        alice.AccountID,
			"device_id":         aliceDevice.DeviceID,
		},
	)

	maxClaims := 1
	activeInvite := createRelaySpaceInvite(
		t,
		server.URL,
		space.RelaySpaceID,
		map[string]any{
			"relay_space_invite_id": "claim-refusal-active",
			"invite_token":          "claim-refusal-active-token",
			"display_code":          "CLAIM-REFUSAL",
			"created_by_member_id":  creator.RoutingMemberID,
			"max_claims":            maxClaims,
		},
	)

	var mismatch errorResponse
	doPost(
		t,
		server.URL+"/v0/relay-spaces/invites/claim",
		map[string]any{
			"invite_token": activeInvite.InviteToken,
			"account_id":   alice.AccountID,
			"device_id":    bobDevice.DeviceID,
		},
		http.StatusConflict,
		&mismatch,
	)
	if mismatch.Error.Code != "account_device_mismatch" {
		t.Fatalf("error code = %q", mismatch.Error.Code)
	}

	var valid db.RelaySpaceInviteClaimResult
	doPost(
		t,
		server.URL+"/v0/relay-spaces/invites/claim",
		map[string]any{
			"invite_token": activeInvite.InviteToken,
			"account_id":   bob.AccountID,
			"device_id":    bobDevice.DeviceID,
		},
		http.StatusCreated,
		&valid,
	)
	if valid.RelaySpaceInvite.ClaimCount != 1 {
		t.Fatalf("claim_count = %d", valid.RelaySpaceInvite.ClaimCount)
	}

	expiredInvite := createRelaySpaceInvite(
		t,
		server.URL,
		space.RelaySpaceID,
		map[string]any{
			"relay_space_invite_id": "claim-refusal-expired",
			"invite_token":          "claim-refusal-expired-token",
			"display_code":          "CLAIM-EXPIRED",
			"created_by_member_id":  creator.RoutingMemberID,
			"expires_at":            "2020-01-01T00:00:00Z",
		},
	)

	var expired errorResponse
	doPost(
		t,
		server.URL+"/v0/relay-spaces/invites/claim",
		map[string]any{
			"invite_token": expiredInvite.InviteToken,
			"account_id":   charlie.AccountID,
			"device_id":    charlieDevice.DeviceID,
		},
		http.StatusGone,
		&expired,
	)
	if expired.Error.Code != "relay_space_invite_expired" {
		t.Fatalf("error code = %q", expired.Error.Code)
	}
}
