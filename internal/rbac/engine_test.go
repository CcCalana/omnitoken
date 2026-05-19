package rbac

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
)

func TestAuthorizeRoleActionMatrix(t *testing.T) {
	t.Parallel()

	actor := testActor()
	expect := map[Role]map[string]bool{
		RoleAdmin: {
			ActionViewOverview:      true,
			ActionViewUsers:         true,
			ActionViewModels:        true,
			ActionViewAuditLogs:     true,
			ActionViewOwnUsage:      true,
			ActionCreateVirtualKey:  true,
			ActionDisableVirtualKey: true,
			ActionUpdateQuota:       true,
		},
		RoleViewer: {
			ActionViewOverview:      true,
			ActionViewUsers:         true,
			ActionViewModels:        true,
			ActionViewAuditLogs:     true,
			ActionViewOwnUsage:      false,
			ActionCreateVirtualKey:  false,
			ActionDisableVirtualKey: false,
			ActionUpdateQuota:       false,
		},
		RoleMember: {
			ActionViewOverview:      false,
			ActionViewUsers:         false,
			ActionViewModels:        false,
			ActionViewAuditLogs:     false,
			ActionViewOwnUsage:      true,
			ActionCreateVirtualKey:  false,
			ActionDisableVirtualKey: false,
			ActionUpdateQuota:       false,
		},
	}

	for role, actions := range expect {
		role := role
		actions := actions
		t.Run(string(role), func(t *testing.T) {
			t.Parallel()

			engine := NewEngine(&fakeStore{lookup: RoleLookup{
				Found:  true,
				Status: StatusActive,
				Roles:  []Role{role},
			}})
			for _, action := range AllActions {
				action := action
				t.Run(action, func(t *testing.T) {
					t.Parallel()

					allowed, reason, err := engine.Authorize(context.Background(), actor, action)
					if err != nil {
						t.Fatalf("authorize: %v", err)
					}
					if allowed != actions[action] {
						t.Fatalf("expected allowed=%v for %s/%s, got %v", actions[action], role, action, allowed)
					}
					wantReason := ReasonActionNotPermitted
					if allowed {
						wantReason = reasonForRole(role)
					}
					if reason != wantReason {
						t.Fatalf("expected reason %q, got %q", wantReason, reason)
					}
				})
			}
		})
	}
}

func TestAuthorizeRejectsUnknownAction(t *testing.T) {
	t.Parallel()

	engine := NewEngine(&fakeStore{lookup: RoleLookup{
		Found:  true,
		Status: StatusActive,
		Roles:  []Role{RoleAdmin},
	}})

	allowed, reason, err := engine.Authorize(context.Background(), testActor(), "unknown_admin_write")
	if err != nil {
		t.Fatalf("authorize: %v", err)
	}
	if allowed || reason != ReasonActionNotPermitted {
		t.Fatalf("expected action_not_permitted deny, got allowed=%v reason=%q", allowed, reason)
	}
}

func TestAuthorizeRejectsUnknownUserAndMissingRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		lookup RoleLookup
	}{
		{name: "user missing", lookup: RoleLookup{}},
		{name: "role missing", lookup: RoleLookup{Found: true, Status: StatusActive}},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			engine := NewEngine(&fakeStore{lookup: tt.lookup})
			allowed, reason, err := engine.Authorize(context.Background(), testActor(), ActionViewOverview)
			if err != nil {
				t.Fatalf("authorize: %v", err)
			}
			if allowed || reason != ReasonRoleNotFound {
				t.Fatalf("expected role_not_found deny, got allowed=%v reason=%q", allowed, reason)
			}
		})
	}
}

func TestAuthorizeRejectsDisabledUser(t *testing.T) {
	t.Parallel()

	engine := NewEngine(&fakeStore{lookup: RoleLookup{
		Found:  true,
		Status: StatusDisabled,
		Roles:  []Role{RoleAdmin},
	}})

	allowed, reason, err := engine.Authorize(context.Background(), testActor(), ActionCreateVirtualKey)
	if err != nil {
		t.Fatalf("authorize: %v", err)
	}
	if allowed || reason != ReasonUserDisabled {
		t.Fatalf("expected user_disabled deny, got allowed=%v reason=%q", allowed, reason)
	}
}

func TestAuthorizeMultiRoleUsesHighestAllowedRole(t *testing.T) {
	t.Parallel()

	engine := NewEngine(&fakeStore{lookup: RoleLookup{
		Found:  true,
		Status: StatusActive,
		Roles:  []Role{RoleMember, RoleViewer},
	}})

	allowed, reason, err := engine.Authorize(context.Background(), testActor(), ActionViewOverview)
	if err != nil {
		t.Fatalf("authorize: %v", err)
	}
	if !allowed || reason != ReasonAllowedByViewer {
		t.Fatalf("expected viewer allow, got allowed=%v reason=%q", allowed, reason)
	}
}

func TestAuthorizeMultiRoleFallsThroughDeniedHigherRole(t *testing.T) {
	t.Parallel()

	const syntheticAction = "synthetic_viewer_only"
	engine := newEngineWithPolicy(&fakeStore{lookup: RoleLookup{
		Found:  true,
		Status: StatusActive,
		Roles:  []Role{RoleAdmin, RoleViewer},
	}}, policy{
		RoleAdmin:  {syntheticAction: false},
		RoleViewer: {syntheticAction: true},
		RoleMember: {},
	})

	allowed, reason, err := engine.Authorize(context.Background(), testActor(), syntheticAction)
	if err != nil {
		t.Fatalf("authorize: %v", err)
	}
	if !allowed || reason != ReasonAllowedByViewer {
		t.Fatalf("expected viewer fall-through allow, got allowed=%v reason=%q", allowed, reason)
	}
}

func TestAuthorizeInvalidActorDoesNotHitStore(t *testing.T) {
	t.Parallel()

	store := &fakeStore{lookup: RoleLookup{Found: true, Status: StatusActive, Roles: []Role{RoleAdmin}}}
	engine := NewEngine(store)

	allowed, reason, err := engine.Authorize(context.Background(), Actor{}, ActionViewOverview)
	if err != nil {
		t.Fatalf("authorize: %v", err)
	}
	if allowed || reason != ReasonInvalidActor {
		t.Fatalf("expected invalid_actor deny, got allowed=%v reason=%q", allowed, reason)
	}
	if calls := store.callCount(); calls != 0 {
		t.Fatalf("store should not be called for invalid actor, got %d calls", calls)
	}
}

func TestAuthorizeLookupError(t *testing.T) {
	t.Parallel()

	lookupErr := errors.New("database unavailable")
	engine := NewEngine(&fakeStore{err: lookupErr})

	allowed, reason, err := engine.Authorize(context.Background(), testActor(), ActionViewOverview)
	if !errors.Is(err, lookupErr) {
		t.Fatalf("expected wrapped lookup error, got %v", err)
	}
	if allowed || reason != ReasonRoleLookupFailed {
		t.Fatalf("expected role_lookup_failed deny, got allowed=%v reason=%q", allowed, reason)
	}
}

func TestNewEngineWithoutStore(t *testing.T) {
	t.Parallel()

	if got := NewEngine(nil); got != nil {
		t.Fatalf("expected nil engine without store, got %#v", got)
	}
	if got := NewEngine(NewPostgresStore(nil)); got != nil {
		t.Fatalf("expected nil engine without postgres db, got %#v", got)
	}
}

func TestNilEngineAuthorizeFailsClosed(t *testing.T) {
	t.Parallel()

	var engine *Engine
	allowed, reason, err := engine.Authorize(context.Background(), testActor(), ActionViewOverview)
	if !errors.Is(err, ErrStoreNotConfigured) {
		t.Fatalf("expected store-not-configured error, got %v", err)
	}
	if allowed || reason != ReasonRoleLookupFailed {
		t.Fatalf("expected role_lookup_failed deny, got allowed=%v reason=%q", allowed, reason)
	}
}

func testActor() Actor {
	return Actor{
		OrgID:  uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		UserID: uuid.MustParse("00000000-0000-0000-0000-000000000201"),
	}
}

type fakeStore struct {
	lookup RoleLookup
	err    error
	mu     sync.Mutex
	calls  int
}

func (s *fakeStore) LookupRoles(context.Context, Actor) (RoleLookup, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.calls++
	if s.err != nil {
		return RoleLookup{}, s.err
	}
	return s.lookup, nil
}

func (s *fakeStore) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}
