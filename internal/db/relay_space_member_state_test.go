package db

import (
	"errors"
	"testing"
)

func TestUpdateRelaySpaceMemberStateLifecycleAndRoutingAuthority(
	t *testing.T,
) {
	store := openMigratedRelaySpaceTestStore(t)
	defer store.Close()

	space, member := seedRelaySpaceMemberStateFixture(t, store)

	disabled, err := store.UpdateRelaySpaceMemberState(
		UpdateRelaySpaceMemberStateInput{
			RelaySpaceID:    space.RelaySpaceID,
			RoutingMemberID: member.RoutingMemberID,
			TargetState:     RelaySpaceMemberStateDisabled,
			ChangedAt:       "2026-07-13T01:00:00Z",
		},
	)
	if err != nil {
		t.Fatalf("disable member: %v", err)
	}
	if disabled.TransitionClassification !=
		RelaySpaceMemberStateTransitioned ||
		disabled.Idempotent ||
		disabled.PreviousState != RelaySpaceMemberStateActive ||
		disabled.CurrentState != RelaySpaceMemberStateDisabled ||
		disabled.RoutingMember.DisabledAt != "2026-07-13T01:00:00Z" {
		t.Fatalf("unexpected disabled result: %+v", disabled)
	}

	active, err := store.IsActiveRelaySpaceDeviceMember(
		space.RelaySpaceID,
		member.DeviceID,
	)
	if err != nil {
		t.Fatalf("active lookup after disable: %v", err)
	}
	if active {
		t.Fatal("disabled member still authorized routing")
	}

	disabledAgain, err := store.UpdateRelaySpaceMemberState(
		UpdateRelaySpaceMemberStateInput{
			RelaySpaceID:    space.RelaySpaceID,
			RoutingMemberID: member.RoutingMemberID,
			TargetState:     RelaySpaceMemberStateDisabled,
			ChangedAt:       "2026-07-13T01:01:00Z",
		},
	)
	if err != nil {
		t.Fatalf("idempotent disable: %v", err)
	}
	if disabledAgain.TransitionClassification !=
		RelaySpaceMemberStateAlreadyCurrent ||
		!disabledAgain.Idempotent ||
		disabledAgain.RoutingMember.DisabledAt !=
			"2026-07-13T01:00:00Z" {
		t.Fatalf("unexpected idempotent disable: %+v", disabledAgain)
	}

	reactivated, err := store.UpdateRelaySpaceMemberState(
		UpdateRelaySpaceMemberStateInput{
			RelaySpaceID:    space.RelaySpaceID,
			RoutingMemberID: member.RoutingMemberID,
			TargetState:     RelaySpaceMemberStateActive,
			ChangedAt:       "2026-07-13T01:02:00Z",
		},
	)
	if err != nil {
		t.Fatalf("reactivate member: %v", err)
	}
	if reactivated.PreviousState != RelaySpaceMemberStateDisabled ||
		reactivated.CurrentState != RelaySpaceMemberStateActive ||
		reactivated.RoutingMember.DisabledAt != "" {
		t.Fatalf("unexpected reactivation result: %+v", reactivated)
	}

	active, err = store.IsActiveRelaySpaceDeviceMember(
		space.RelaySpaceID,
		member.DeviceID,
	)
	if err != nil {
		t.Fatalf("active lookup after reactivation: %v", err)
	}
	if !active {
		t.Fatal("reactivated member did not regain routing authority")
	}

	left, err := store.UpdateRelaySpaceMemberState(
		UpdateRelaySpaceMemberStateInput{
			RelaySpaceID:    space.RelaySpaceID,
			RoutingMemberID: member.RoutingMemberID,
			TargetState:     RelaySpaceMemberStateLeft,
			ChangedAt:       "2026-07-13T01:03:00Z",
		},
	)
	if err != nil {
		t.Fatalf("leave member: %v", err)
	}
	if left.CurrentState != RelaySpaceMemberStateLeft ||
		left.RoutingMember.DisabledAt != "" {
		t.Fatalf("unexpected leave result: %+v", left)
	}

	active, err = store.IsActiveRelaySpaceDeviceMember(
		space.RelaySpaceID,
		member.DeviceID,
	)
	if err != nil {
		t.Fatalf("active lookup after leave: %v", err)
	}
	if active {
		t.Fatal("left member still authorized routing")
	}

	leftAgain, err := store.UpdateRelaySpaceMemberState(
		UpdateRelaySpaceMemberStateInput{
			RelaySpaceID:    space.RelaySpaceID,
			RoutingMemberID: member.RoutingMemberID,
			TargetState:     RelaySpaceMemberStateLeft,
			ChangedAt:       "2026-07-13T01:04:00Z",
		},
	)
	if err != nil {
		t.Fatalf("idempotent leave: %v", err)
	}
	if !leftAgain.Idempotent ||
		leftAgain.TransitionClassification !=
			RelaySpaceMemberStateAlreadyCurrent {
		t.Fatalf("unexpected idempotent leave: %+v", leftAgain)
	}

	_, err = store.UpdateRelaySpaceMemberState(
		UpdateRelaySpaceMemberStateInput{
			RelaySpaceID:    space.RelaySpaceID,
			RoutingMemberID: member.RoutingMemberID,
			TargetState:     RelaySpaceMemberStateActive,
			ChangedAt:       "2026-07-13T01:05:00Z",
		},
	)
	if !errors.Is(err, ErrRelaySpaceMemberRejoinRequired) {
		t.Fatalf("left-to-active err = %v", err)
	}
}

func TestUpdateRelaySpaceMemberStateRefusesWrongSpaceUnsupportedAndInconsistent(
	t *testing.T,
) {
	store := openMigratedRelaySpaceTestStore(t)
	defer store.Close()

	space, member := seedRelaySpaceMemberStateFixture(t, store)

	_, err := store.UpdateRelaySpaceMemberState(
		UpdateRelaySpaceMemberStateInput{
			RelaySpaceID:    "other-space",
			RoutingMemberID: member.RoutingMemberID,
			TargetState:     RelaySpaceMemberStateDisabled,
		},
	)
	if !errors.Is(err, ErrRelaySpaceMemberWrongSpace) {
		t.Fatalf("wrong-space err = %v", err)
	}

	_, err = store.UpdateRelaySpaceMemberState(
		UpdateRelaySpaceMemberStateInput{
			RelaySpaceID:    space.RelaySpaceID,
			RoutingMemberID: member.RoutingMemberID,
			TargetState:     "removed",
		},
	)
	if !errors.Is(err, ErrRelaySpaceMemberTargetStateUnsupported) {
		t.Fatalf("unsupported-target err = %v", err)
	}

	_, err = store.UpdateRelaySpaceMemberState(
		UpdateRelaySpaceMemberStateInput{
			RelaySpaceID:    space.RelaySpaceID,
			RoutingMemberID: "missing-member",
			TargetState:     RelaySpaceMemberStateDisabled,
		},
	)
	if !errors.Is(err, ErrRelaySpaceMemberNotFound) {
		t.Fatalf("missing-member err = %v", err)
	}

	_, err = store.DB.Exec(
		`UPDATE relay_space_members
		    SET disabled_at = ?
		  WHERE routing_member_id = ?`,
		"2026-07-13T01:10:00Z",
		member.RoutingMemberID,
	)
	if err != nil {
		t.Fatalf("seed inconsistent member: %v", err)
	}

	_, err = store.UpdateRelaySpaceMemberState(
		UpdateRelaySpaceMemberStateInput{
			RelaySpaceID:    space.RelaySpaceID,
			RoutingMemberID: member.RoutingMemberID,
			TargetState:     RelaySpaceMemberStateDisabled,
		},
	)
	if !errors.Is(err, ErrRelaySpaceMemberStateInconsistent) {
		t.Fatalf("inconsistent-state err = %v", err)
	}
}

func TestUpdateRelaySpaceMemberStateAllowsDisabledToLeftWithoutReactivation(
	t *testing.T,
) {
	store := openMigratedRelaySpaceTestStore(t)
	defer store.Close()

	space, member := seedRelaySpaceMemberStateFixture(t, store)

	_, err := store.UpdateRelaySpaceMemberState(
		UpdateRelaySpaceMemberStateInput{
			RelaySpaceID:    space.RelaySpaceID,
			RoutingMemberID: member.RoutingMemberID,
			TargetState:     RelaySpaceMemberStateDisabled,
			ChangedAt:       "2026-07-13T01:20:00Z",
		},
	)
	if err != nil {
		t.Fatalf("disable member: %v", err)
	}

	left, err := store.UpdateRelaySpaceMemberState(
		UpdateRelaySpaceMemberStateInput{
			RelaySpaceID:    space.RelaySpaceID,
			RoutingMemberID: member.RoutingMemberID,
			TargetState:     RelaySpaceMemberStateLeft,
			ChangedAt:       "2026-07-13T01:21:00Z",
		},
	)
	if err != nil {
		t.Fatalf("disabled to left: %v", err)
	}
	if left.PreviousState != RelaySpaceMemberStateDisabled ||
		left.CurrentState != RelaySpaceMemberStateLeft ||
		left.RoutingMember.DisabledAt != "" {
		t.Fatalf("unexpected disabled-to-left result: %+v", left)
	}
}

func seedRelaySpaceMemberStateFixture(
	t *testing.T,
	store *Store,
) (RelaySpace, RelaySpaceMember) {
	t.Helper()

	seedRelaySpaceAccountAndDevice(t, store)

	space, err := store.CreateRelaySpace(CreateRelaySpaceInput{
		RelaySpaceID:       "member-state-space",
		DisplayLabel:       "member state space",
		CreatedByAccountID: "account-1",
		CreatedByDeviceID:  "device-1",
	})
	if err != nil {
		t.Fatalf("create relay space: %v", err)
	}

	member, err := store.RegisterRelaySpaceMember(
		RegisterRelaySpaceMemberInput{
			RoutingMemberID: "member-state-member",
			RelaySpaceID:    space.RelaySpaceID,
			AccountID:       "account-1",
			DeviceID:        "device-1",
			DisplayLabel:    "member state member",
		},
	)
	if err != nil {
		t.Fatalf("register member: %v", err)
	}

	return space, member
}
