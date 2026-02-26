package database

// GetAppState retrieves a value from the app_state table.
// Returns ("", nil) when the key does not exist.
func (db *Database) GetAppState(key string) (string, error) {
	var value string
	err := db.db.QueryRow(
		`SELECT value FROM app_state WHERE key = ?`, key,
	).Scan(&value)
	if err != nil {
		// Treat missing row as empty — not an error for the caller.
		return "", nil
	}
	return value, nil
}

// SetAppState inserts or replaces a key/value pair in the app_state table.
func (db *Database) SetAppState(key, value string) error {
	_, err := db.db.Exec(
		`INSERT INTO app_state (key, value) VALUES (?, ?)
         ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	return err
}
