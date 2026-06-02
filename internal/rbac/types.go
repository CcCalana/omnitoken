package rbac

import (
	"errors"

	"github.com/google/uuid"
)

const (
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
	RoleViewer Role = "viewer"

	StatusActive   = "active"
	StatusDisabled = "disabled"

	ActionViewOverview       = "view_overview"
	ActionViewUsers          = "view_users"
	ActionViewModels         = "view_models"
	ActionViewAuditLogs      = "view_audit_logs"
	ActionViewOwnUsage       = "view_own_usage"
	ActionCreateUser         = "create_user"
	ActionCreateVirtualKey   = "create_virtual_key"
	ActionDisableVirtualKey  = "disable_virtual_key"
	ActionCreateCredential   = "create_provider_credential"
	ActionDisableCredential  = "disable_provider_credential"
	ActionCreateVirtualModel = "create_virtual_model"
	ActionUpdateVirtualModel = "update_virtual_model"
	ActionUpdateQuota        = "update_quota"

	ReasonAllowedByAdmin     = "allowed_by_admin"
	ReasonAllowedByViewer    = "allowed_by_viewer"
	ReasonAllowedByMember    = "allowed_by_member"
	ReasonRoleNotFound       = "role_not_found"
	ReasonUserDisabled       = "user_disabled"
	ReasonActionNotPermitted = "action_not_permitted"
	ReasonInvalidActor       = "invalid_actor"
	ReasonRoleLookupFailed   = "role_lookup_failed"
)

var ErrStoreNotConfigured = errors.New("rbac store not configured")

type Role string

type Actor struct {
	OrgID  uuid.UUID
	UserID uuid.UUID
}

type RoleLookup struct {
	Found  bool
	Status string
	Roles  []Role
}

var AllActions = []string{
	ActionViewOverview,
	ActionViewUsers,
	ActionViewModels,
	ActionViewAuditLogs,
	ActionViewOwnUsage,
	ActionCreateUser,
	ActionCreateVirtualKey,
	ActionDisableVirtualKey,
	ActionCreateCredential,
	ActionDisableCredential,
	ActionCreateVirtualModel,
	ActionUpdateVirtualModel,
	ActionUpdateQuota,
}
