package rbac

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/google/uuid"
)

type Store interface {
	LookupRoles(context.Context, Actor) (RoleLookup, error)
}

type Engine struct {
	store  Store
	policy policy
}

func NewEngine(store Store) *Engine {
	if store == nil || isNilStore(store) {
		return nil
	}
	return &Engine{store: store, policy: defaultPolicy}
}

func (e *Engine) Authorize(ctx context.Context, actor Actor, action string) (bool, string, error) {
	if actor.OrgID == uuid.Nil || actor.UserID == uuid.Nil {
		return false, ReasonInvalidActor, nil
	}
	action = strings.TrimSpace(action)
	if action == "" {
		return false, ReasonActionNotPermitted, nil
	}
	if e == nil || e.store == nil {
		return false, ReasonRoleLookupFailed, ErrStoreNotConfigured
	}

	lookup, err := e.store.LookupRoles(ctx, actor)
	if err != nil {
		return false, ReasonRoleLookupFailed, fmt.Errorf("lookup rbac roles: %w", err)
	}
	if !lookup.Found || len(lookup.Roles) == 0 {
		return false, ReasonRoleNotFound, nil
	}
	if lookup.Status != StatusActive {
		return false, ReasonUserDisabled, nil
	}

	assigned := assignedRoles(lookup.Roles)
	for _, role := range rolePriority {
		if !assigned[role] {
			continue
		}
		// Keep OR semantics across multiple roles: a denial by a higher-priority
		// role must not hide a lower-priority role that explicitly permits a
		// future action.
		if e.policy.allows(role, action) {
			return true, reasonForRole(role), nil
		}
	}
	return false, ReasonActionNotPermitted, nil
}

func assignedRoles(roles []Role) map[Role]bool {
	assigned := make(map[Role]bool, len(roles))
	for _, role := range roles {
		assigned[role] = true
	}
	return assigned
}

func isNilStore(store Store) bool {
	value := reflect.ValueOf(store)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func newEngineWithPolicy(store Store, p policy) *Engine {
	if store == nil || isNilStore(store) {
		return nil
	}
	return &Engine{store: store, policy: p}
}
