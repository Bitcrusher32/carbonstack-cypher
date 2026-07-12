package db

import (
	"errors"
	"strings"
	"sync"
	"testing"
)

func TestClaimRelaySpaceInviteCreatesMemberConsumesClaimAndIsIdempotent(t *testing.T) {
	store := openMigratedRelaySpaceTestStore(t)
	defer store.Close()
	seedRelaySpaceClaimFixture(t, store)

	result, err := store.ClaimRelaySpaceInvite(ClaimRelaySpaceInviteInput{
		InviteToken:  "claim-token",
		AccountID:    "account-2",
		DeviceID:     "device-2",
		DisplayLabel: "Bob",
		ClaimedAt:    "2026-07-12T20:00:00Z",
	})
	if err != nil {
		t.Fatalf("claim relay space invite: %v", err)
	}
	if result.ClaimClassification != RelaySpaceInviteClaimCreated {
		t.Fatalf("classification = %q", result.ClaimClassification)
	}
	if result.Idempotent || !result.ClaimConsumed {
		t.Fatalf(
			"idempotent=%v claim_consumed=%v",
			result.Idempotent,
			result.ClaimConsumed,
		)
	}
	if result.RelaySpaceInvite.ClaimCount != 1 ||
		result.RelaySpaceInvite.State != RelaySpaceInviteStateClaimed {
		t.Fatalf("unexpected invite after claim: %+v", result.RelaySpaceInvite)
	}

	retry, err := store.ClaimRelaySpaceInvite(ClaimRelaySpaceInviteInput{
		InviteToken: "claim-token",
		AccountID:   "account-2",
		DeviceID:    "device-2",
		ClaimedAt:   "2026-07-12T20:01:00Z",
	})
	if err != nil {
		t.Fatalf("idempotent retry: %v", err)
	}
	if retry.ClaimClassification != RelaySpaceInviteClaimAlreadyActive ||
		!retry.Idempotent ||
		retry.ClaimConsumed ||
		retry.RelaySpaceInvite.ClaimCount != 1 {
		t.Fatalf("unexpected retry result: %+v", retry)
	}
}

func TestClaimRelaySpaceInviteRefusesInvalidExpiryMismatchAndMemberState(t *testing.T) {
	store := openMigratedRelaySpaceTestStore(t)
	defer store.Close()
	seedRelaySpaceClaimFixture(t, store)

	_, err := store.ClaimRelaySpaceInvite(ClaimRelaySpaceInviteInput{
		InviteToken: "missing-token",
		AccountID:   "account-2",
		DeviceID:    "device-2",
		ClaimedAt:   "2026-07-12T20:00:00Z",
	})
	if !errors.Is(err, ErrRelaySpaceInviteNotFound) {
		t.Fatalf("missing token err = %v", err)
	}

	_, err = store.ClaimRelaySpaceInvite(ClaimRelaySpaceInviteInput{
		InviteToken: "claim-token",
		AccountID:   "account-1",
		DeviceID:    "device-2",
		ClaimedAt:   "2026-07-12T20:00:00Z",
	})
	if !errors.Is(err, ErrRelaySpaceAccountDeviceMismatch) {
		t.Fatalf("mismatch err = %v", err)
	}

	_, err = store.CreateRelaySpaceInvite(CreateRelaySpaceInviteInput{
		RelaySpaceInviteID: "expired-invite",
		RelaySpaceID:       "claim-space",
		InviteToken:        "expired-token",
		DisplayCode:        "EXPIRED",
		CreatedByMemberID:  "creator-member",
		ExpiresAt:          "2026-07-12T19:59:59Z",
	})
	if err != nil {
		t.Fatalf("create expired invite: %v", err)
	}
	_, err = store.ClaimRelaySpaceInvite(ClaimRelaySpaceInviteInput{
		InviteToken: "expired-token",
		AccountID:   "account-2",
		DeviceID:    "device-2",
		ClaimedAt:   "2026-07-12T20:00:00Z",
	})
	if !errors.Is(err, ErrRelaySpaceInviteExpired) {
		t.Fatalf("expired err = %v", err)
	}

	_, err = store.RegisterRelaySpaceMember(RegisterRelaySpaceMemberInput{
		RoutingMemberID: "disabled-existing",
		RelaySpaceID:    "claim-space",
		AccountID:       "account-2",
		DeviceID:        "device-2",
		State:           RelaySpaceMemberStateDisabled,
	})
	if err != nil {
		t.Fatalf("register disabled member: %v", err)
	}
	_, err = store.ClaimRelaySpaceInvite(ClaimRelaySpaceInviteInput{
		InviteToken: "claim-token",
		AccountID:   "account-2",
		DeviceID:    "device-2",
		ClaimedAt:   "2026-07-12T20:00:00Z",
	})
	if !errors.Is(err, ErrRelaySpaceMemberDisabled) {
		t.Fatalf("disabled member err = %v", err)
	}

	invite, err := store.GetRelaySpaceInviteByTokenHash(HashInviteCode("claim-token"))
	if err != nil {
		t.Fatalf("get invite: %v", err)
	}
	if invite.ClaimCount != 0 {
		t.Fatalf("claim_count = %d, want 0", invite.ClaimCount)
	}
}

func TestClaimRelaySpaceInviteSerializesSingleUseConcurrentClaims(t *testing.T) {
	store := openMigratedRelaySpaceTestStore(t)
	defer store.Close()
	seedRelaySpaceClaimFixture(t, store)
	seedThirdRelaySpaceClaimAccountAndDevice(t, store)

	type outcome struct {
		result RelaySpaceInviteClaimResult
		err    error
	}
	start := make(chan struct{})
	outcomes := make(chan outcome, 2)
	var wait sync.WaitGroup

	for _, identity := range []struct {
		accountID string
		deviceID  string
	}{
		{accountID: "account-2", deviceID: "device-2"},
		{accountID: "account-3", deviceID: "device-3"},
	} {
		wait.Add(1)
		go func(accountID string, deviceID string) {
			defer wait.Done()
			<-start
			result, err := store.ClaimRelaySpaceInvite(ClaimRelaySpaceInviteInput{
				InviteToken: "claim-token",
				AccountID:   accountID,
				DeviceID:    deviceID,
				ClaimedAt:   "2026-07-12T20:00:00Z",
			})
			outcomes <- outcome{result: result, err: err}
		}(identity.accountID, identity.deviceID)
	}

	close(start)
	wait.Wait()
	close(outcomes)

	successes := 0
	exhausted := 0
	for got := range outcomes {
		switch {
		case got.err == nil:
			successes++
		case errors.Is(got.err, ErrRelaySpaceInviteExhausted):
			exhausted++
		default:
			t.Fatalf("unexpected concurrent result: %v", got.err)
		}
	}
	if successes != 1 || exhausted != 1 {
		t.Fatalf("successes=%d exhausted=%d, want 1/1", successes, exhausted)
	}

	invite, err := store.GetRelaySpaceInviteByTokenHash(HashInviteCode("claim-token"))
	if err != nil {
		t.Fatalf("get invite: %v", err)
	}
	if invite.ClaimCount != 1 ||
		invite.State != RelaySpaceInviteStateClaimed {
		t.Fatalf("unexpected final invite: %+v", invite)
	}

	members, err := store.ListRelaySpaceMembers("claim-space")
	if err != nil {
		t.Fatalf("list members: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("member count = %d, want creator plus one claimant", len(members))
	}
}

func seedRelaySpaceClaimFixture(t *testing.T, store *Store) {
	t.Helper()
	seedRelaySpaceAccountAndDevice(t, store)
	seedSecondRelaySpaceAccountAndDevice(t, store)

	_, err := store.CreateRelaySpace(CreateRelaySpaceInput{
		RelaySpaceID:       "claim-space",
		DisplayLabel:       "claim space",
		CreatedByAccountID: "account-1",
		CreatedByDeviceID:  "device-1",
	})
	if err != nil {
		t.Fatalf("create relay space: %v", err)
	}
	_, err = store.RegisterRelaySpaceMember(RegisterRelaySpaceMemberInput{
		RoutingMemberID: "creator-member",
		RelaySpaceID:    "claim-space",
		AccountID:       "account-1",
		DeviceID:        "device-1",
		DisplayLabel:    "creator",
	})
	if err != nil {
		t.Fatalf("register creator: %v", err)
	}

	maxClaims := 1
	_, err = store.CreateRelaySpaceInvite(CreateRelaySpaceInviteInput{
		RelaySpaceInviteID: "claim-invite",
		RelaySpaceID:       "claim-space",
		InviteToken:        "claim-token",
		DisplayCode:        "CLAIM-ONE",
		CreatedByMemberID:  "creator-member",
		MaxClaims:          &maxClaims,
	})
	if err != nil {
		t.Fatalf("create claim invite: %v", err)
	}
}

func seedThirdRelaySpaceClaimAccountAndDevice(t *testing.T, store *Store) {
	t.Helper()
	_, err := store.DB.Exec(
		"INSERT INTO accounts (account_id, display_name, created_at) VALUES (?, ?, ?)",
		"account-3",
		"Charlie",
		"2026-07-12T00:00:00Z",
	)
	if err != nil {
		t.Fatalf("insert third account: %v", err)
	}
	_, err = store.DB.Exec(
		"INSERT INTO devices (device_id, account_id, device_label, public_identity_key, public_prekey_bundle, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		"device-3",
		"account-3",
		"charlie-device",
		"charlie-public-identity-key",
		"charlie-prekey-bundle",
		"2026-07-12T00:00:00Z",
	)
	if err != nil {
		t.Fatalf("insert third device: %v", err)
	}
}

func TestRelaySpaceInviteClaimErrorsRemainRoutingOnly(t *testing.T) {
	for _, value := range []string{
		ErrRelaySpaceInviteDisabled.Error(),
		ErrRelaySpaceInviteExpired.Error(),
		ErrRelaySpaceInviteExhausted.Error(),
		ErrRelaySpaceMemberDisabled.Error(),
		ErrRelaySpaceMemberLeft.Error(),
	} {
		lowered := strings.ToLower(value)
		for _, forbidden := range []string{
			"verified",
			"trust",
			"openmls",
			"identity proof",
		} {
			if strings.Contains(lowered, forbidden) {
				t.Fatalf("routing-only error %q contains %q", value, forbidden)
			}
		}
	}
}
