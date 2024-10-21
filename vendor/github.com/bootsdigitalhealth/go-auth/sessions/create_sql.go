package sessions

import (
	"database/sql"
	"strconv"
)

func getUserRoles(db *sql.DB, userID int64) (map[string]string, error) {
	var roleID int
	var name string
	roleMap := make(map[string]string)

	rows, err := db.Query(getUserRolesSQL(), userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&roleID, &name)
		if err != nil {
			return nil, err
		}
		roleMap[strconv.Itoa(roleID)] = name
	}
	err = rows.Err()
	if err != nil {
		return nil, err
	}
	return roleMap, nil
}

func getUserRolesSQL() string {
	return `
	SELECT 
		r.role_id,
		r.name
	FROM users u 
	INNER JOIN user_roles_xref urx 
		USING (user_id) 
	INNER JOIN roles r 
		USING (role_id) 
	WHERE u.user_id = ?`
}

func GetUserForAuth(db *sql.DB, username string, systemCode string) (*userForAuth, error) {
	user := userForAuth{}
	rows, err := db.Query(GetUserForAuthSQL(), username, systemCode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&user.UserID, &user.Status, &user.StoredPassword, &user.Created)
		if err != nil {
			return nil, err
		}
	}
	err = rows.Err()
	if err != nil {
		return nil, err
	}
	if user.UserID == 0 {
		return nil, UserNotFound{}
	}
	return &user, nil
}

func GetUserForAuthSQL() string {
	return "SELECT user_id, status, password, created FROM users WHERE username = ? AND system_code = ?"
}
