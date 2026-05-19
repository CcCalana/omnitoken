package rbac

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	if db == nil {
		return nil
	}
	return &PostgresStore{db: db}
}

func (s *PostgresStore) LookupRoles(ctx context.Context, actor Actor) (RoleLookup, error) {
	if s == nil || s.db == nil {
		return RoleLookup{}, ErrStoreNotConfigured
	}

	rows, err := s.db.QueryContext(ctx, lookupRolesSQL, actor.OrgID, actor.UserID)
	if err != nil {
		return RoleLookup{}, fmt.Errorf("query rbac roles: %w", err)
	}
	defer rows.Close()

	lookup := RoleLookup{}
	for rows.Next() {
		var status string
		var role sql.NullString
		if err := rows.Scan(&status, &role); err != nil {
			return RoleLookup{}, fmt.Errorf("scan rbac role: %w", err)
		}
		lookup.Found = true
		lookup.Status = status
		if role.Valid {
			canonical := strings.TrimSpace(role.String)
			if canonical != "" {
				lookup.Roles = append(lookup.Roles, Role(canonical))
			}
		}
	}
	if err := rows.Err(); err != nil {
		return RoleLookup{}, fmt.Errorf("iterate rbac roles: %w", err)
	}
	return lookup, nil
}

const lookupRolesSQL = `
SELECT
  u.status,
  r.canonical_name
FROM users u
LEFT JOIN role_assignments ra
  ON ra.organization_id = u.organization_id
 AND ra.user_id = u.id
LEFT JOIN roles r
  ON r.id = ra.role_id
WHERE u.organization_id = $1
  AND u.id = $2
ORDER BY r.canonical_name`
