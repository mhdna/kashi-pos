package main

import (
	"fmt"

	db "kashi-pos/db/sqlc"
)

func (a *App) ListUsers() ([]map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	users, err := a.store.ListUsers(a.ctx)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, len(users))
	for i, u := range users {
		role, _ := a.store.GetRoleByID(a.ctx, u.RoleID)
		roleName := ""
		if role.Name != "" {
			roleName = role.Name
		}
		result[i] = map[string]any{
			"id":               u.ID,
			"username":         u.Username,
			"roleId":           u.RoleID,
			"role":             roleName,
			"active":           u.Active,
			"attendanceUserId": u.AttendanceUserID,
			// A synced user's roster (username/role/active/akuvox id) is
			// kashi-owned and read-only here; only its PIN can be reset
			// locally.
			"isSynced": u.RemoteID.Valid,
		}
	}
	return result, nil
}

func (a *App) UpdateUserAttendanceID(id int64, attendanceUserID string) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	return a.store.UpdateUserAttendanceID(a.ctx, db.UpdateUserAttendanceIDParams{ID: id, AttendanceUserID: attendanceUserID})
}

func (a *App) CreateUser(username, password string, roleID int64) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	user, err := a.store.CreateUser(a.ctx, db.CreateUserParams{
		Username:     username,
		PasswordHash: hashPassword(password),
		RoleID:       roleID,
		Active:       1,
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"id":       user.ID,
		"username": user.Username,
		"roleId":   user.RoleID,
		"active":   user.Active,
	}, nil
}

func (a *App) UpdateUser(id int64, username, password string, roleID, active int64) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	params := db.UpdateUserParams{
		ID:       id,
		Username: username,
		RoleID:   roleID,
		Active:   active,
	}
	if password != "" {
		params.PasswordHash = hashPassword(password)
	} else {
		user, err := a.store.GetUserByID(a.ctx, id)
		if err != nil {
			return err
		}
		params.PasswordHash = user.PasswordHash
	}
	_, err := a.store.UpdateUser(a.ctx, params)
	return err
}

func (a *App) DeleteUser(id int64) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	return a.store.DeleteUser(a.ctx, id)
}

func (a *App) ListRoles() ([]map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	roles, err := a.store.ListRoles(a.ctx)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, len(roles))
	for i, r := range roles {
		result[i] = map[string]any{
			"id":          r.ID,
			"name":        r.Name,
			"description": r.Description,
		}
	}
	return result, nil
}
