package rbac

type policy map[Role]map[string]bool

var defaultPolicy = policy{
	RoleAdmin: {
		ActionViewOverview:       true,
		ActionViewUsers:          true,
		ActionViewModels:         true,
		ActionViewAuditLogs:      true,
		ActionViewOwnUsage:       true,
		ActionCreateUser:         true,
		ActionCreateVirtualKey:   true,
		ActionDisableVirtualKey:  true,
		ActionCreateCredential:   true,
		ActionDisableCredential:  true,
		ActionCreateVirtualModel: true,
		ActionUpdateVirtualModel: true,
		ActionUpdateQuota:        true,
	},
	RoleViewer: {
		ActionViewOverview:       true,
		ActionViewUsers:          true,
		ActionViewModels:         true,
		ActionViewAuditLogs:      true,
		ActionViewOwnUsage:       false,
		ActionCreateUser:         false,
		ActionCreateVirtualKey:   false,
		ActionDisableVirtualKey:  false,
		ActionCreateCredential:   false,
		ActionDisableCredential:  false,
		ActionCreateVirtualModel: false,
		ActionUpdateVirtualModel: false,
		ActionUpdateQuota:        false,
	},
	RoleMember: {
		ActionViewOverview:       false,
		ActionViewUsers:          false,
		ActionViewModels:         false,
		ActionViewAuditLogs:      false,
		ActionViewOwnUsage:       true,
		ActionCreateUser:         false,
		ActionCreateVirtualKey:   false,
		ActionDisableVirtualKey:  false,
		ActionCreateCredential:   false,
		ActionDisableCredential:  false,
		ActionCreateVirtualModel: false,
		ActionUpdateVirtualModel: false,
		ActionUpdateQuota:        false,
	},
}

var rolePriority = []Role{RoleAdmin, RoleViewer, RoleMember}

func (p policy) allows(role Role, action string) bool {
	actions, ok := p[role]
	if !ok {
		return false
	}
	allowed, ok := actions[action]
	return ok && allowed
}

func reasonForRole(role Role) string {
	switch role {
	case RoleAdmin:
		return ReasonAllowedByAdmin
	case RoleViewer:
		return ReasonAllowedByViewer
	case RoleMember:
		return ReasonAllowedByMember
	default:
		return ReasonActionNotPermitted
	}
}
