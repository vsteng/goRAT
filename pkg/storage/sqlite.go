package storage

import (
	"database/sql"
	"encoding/json"
	"log"
	"strings"
	"sync"
	"time"

	"gorat/common"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStore implements Store interface using SQLite backend
type SQLiteStore struct {
	db *sql.DB
	mu sync.RWMutex
}

// NewSQLiteStore creates a new SQLite-backed store
func NewSQLiteStore(dbPath string) (Store, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	store := &SQLiteStore{
		db: db,
	}

	if err := store.initDB(); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

// initDB initializes the database schema
func (s *SQLiteStore) initDB() error {
	schema := `
	CREATE TABLE IF NOT EXISTS clients (
		id TEXT PRIMARY KEY,
		hostname TEXT,
		os TEXT,
		arch TEXT,
		ip TEXT,
		public_ip TEXT,
		alias TEXT,
		status TEXT,
		client_version TEXT DEFAULT '1.0.0',
		last_seen DATETIME,
		first_seen DATETIME,
		metadata TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_last_seen ON clients(last_seen DESC);
	CREATE INDEX IF NOT EXISTS idx_status ON clients(status);

	CREATE TABLE IF NOT EXISTS proxies (
		id TEXT PRIMARY KEY,
		client_id TEXT NOT NULL,
		local_port INTEGER NOT NULL,
		remote_host TEXT NOT NULL,
		remote_port INTEGER NOT NULL,
		protocol TEXT DEFAULT 'tcp',
		status TEXT DEFAULT 'active',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (client_id) REFERENCES clients(id),
		UNIQUE(client_id, local_port)
	);

	CREATE INDEX IF NOT EXISTS idx_client_proxies ON proxies(client_id);
	CREATE INDEX IF NOT EXISTS idx_proxy_local_port ON proxies(local_port);

	CREATE TABLE IF NOT EXISTS web_users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		full_name TEXT,
		role TEXT DEFAULT 'user',
		status TEXT DEFAULT 'active',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_login DATETIME
	);

	CREATE INDEX IF NOT EXISTS idx_web_users_username ON web_users(username);

	CREATE TABLE IF NOT EXISTS server_settings (
		key TEXT PRIMARY KEY,
		value TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`

	_, err := s.db.Exec(schema)
	if err != nil {
		return err
	}

	// Migration: Add client_version column if it doesn't exist
	migrationSQL := `
	ALTER TABLE clients ADD COLUMN client_version TEXT DEFAULT '1.0.0';
	`

	// Try to run migration, ignore if column already exists
	_, err = s.db.Exec(migrationSQL)
	if err != nil && !strings.Contains(err.Error(), "duplicate column") {
		log.Printf("[WARN] client_version migration failed: %v (this is OK if column already exists)", err)
	}

	// Run migrations for existing databases
	return s.runMigrations()
}

// runMigrations handles database schema migrations for existing databases
func (s *SQLiteStore) runMigrations() error {
	// Check if alias column exists in clients table, add it if not
	rows, err := s.db.Query("PRAGMA table_info(clients)")
	if err != nil {
		// Table might not exist yet (new database), no migration needed
		return nil
	}
	defer rows.Close()

	hasAlias := false
	for rows.Next() {
		var cid int
		var name string
		var type_ string
		var notnull int
		var dflt_value interface{}
		var pk int

		err := rows.Scan(&cid, &name, &type_, &notnull, &dflt_value, &pk)
		if err != nil {
			continue
		}

		if name == "alias" {
			hasAlias = true
			break
		}
	}

	if !hasAlias {
		// Add alias column to existing table
		_, err := s.db.Exec("ALTER TABLE clients ADD COLUMN alias TEXT DEFAULT ''")
		if err != nil {
			log.Printf("Migration warning: Could not add alias column: %v (may already exist)", err)
		}
	}

	return nil
}

// SaveClient saves or updates a client in the database
func (s *SQLiteStore) SaveClient(metadata *common.ClientMetadata) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Serialize metadata to JSON for flexible storage
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	// Try with alias column first, fall back to without if it doesn't exist
	query := `
	INSERT INTO clients (id, hostname, os, arch, ip, public_ip, alias, status, client_version, last_seen, first_seen, metadata, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	ON CONFLICT(id) DO UPDATE SET
		hostname = excluded.hostname,
		os = excluded.os,
		arch = excluded.arch,
		ip = excluded.ip,
		public_ip = excluded.public_ip,
		alias = excluded.alias,
		status = excluded.status,
		client_version = excluded.client_version,
		last_seen = excluded.last_seen,
		metadata = excluded.metadata,
		updated_at = CURRENT_TIMESTAMP
	`

	_, err = s.db.Exec(query,
		metadata.ID,
		metadata.Hostname,
		metadata.OS,
		metadata.Arch,
		metadata.IP,
		metadata.PublicIP,
		metadata.Alias,
		metadata.Status,
		metadata.Version,
		metadata.LastSeen,
		metadata.LastSeen, // first_seen only set on insert
		string(metadataJSON),
	)

	// If alias column doesn't exist, try without it
	if err != nil && strings.Contains(err.Error(), "no column named alias") {
		query := `
		INSERT INTO clients (id, hostname, os, arch, ip, public_ip, status, last_seen, first_seen, metadata, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			hostname = excluded.hostname,
			os = excluded.os,
			arch = excluded.arch,
			ip = excluded.ip,
			public_ip = excluded.public_ip,
			status = excluded.status,
			last_seen = excluded.last_seen,
			metadata = excluded.metadata,
			updated_at = CURRENT_TIMESTAMP
		`

		_, err = s.db.Exec(query,
			metadata.ID,
			metadata.Hostname,
			metadata.OS,
			metadata.Arch,
			metadata.IP,
			metadata.PublicIP,
			metadata.Status,
			metadata.LastSeen,
			metadata.LastSeen, // first_seen only set on insert
			string(metadataJSON),
		)
	}

	return err
}

// GetClient retrieves a client by ID
func (s *SQLiteStore) GetClient(id string) (*common.ClientMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var metadata common.ClientMetadata
	var metadataJSON string

	query := `SELECT id, hostname, os, arch, ip, public_ip, alias, status, last_seen, metadata FROM clients WHERE id = ?`
	err := s.db.QueryRow(query, id).Scan(
		&metadata.ID,
		&metadata.Hostname,
		&metadata.OS,
		&metadata.Arch,
		&metadata.IP,
		&metadata.PublicIP,
		&metadata.Alias,
		&metadata.Status,
		&metadata.LastSeen,
		&metadataJSON,
	)

	if err != nil {
		return nil, err
	}

	return &metadata, nil
}

// GetAllClients retrieves all clients, ordered by last_seen DESC
func (s *SQLiteStore) GetAllClients() ([]*common.ClientMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Use COALESCE to handle alias column gracefully if it doesn't exist in older databases
	query := `SELECT id, hostname, os, arch, ip, public_ip, COALESCE(alias, ''), status, last_seen, metadata 
	          FROM clients 
	          ORDER BY last_seen DESC`

	rows, err := s.db.Query(query)
	if err != nil {
		log.Printf("GetAllClients query error: %v", err)
		// If alias column doesn't exist, try without it
		if strings.Contains(err.Error(), "no such column: alias") {
			query = `SELECT id, hostname, os, arch, ip, public_ip, '', status, last_seen, metadata 
			          FROM clients 
			          ORDER BY last_seen DESC`
			rows, err = s.db.Query(query)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	defer rows.Close()

	var clients []*common.ClientMetadata
	for rows.Next() {
		var metadata common.ClientMetadata
		var metadataJSON string

		err := rows.Scan(
			&metadata.ID,
			&metadata.Hostname,
			&metadata.OS,
			&metadata.Arch,
			&metadata.IP,
			&metadata.PublicIP,
			&metadata.Alias,
			&metadata.Status,
			&metadata.LastSeen,
			&metadataJSON,
		)

		if err != nil {
			log.Printf("Error scanning client row: %v", err)
			continue
		}

		clients = append(clients, &metadata)
	}

	return clients, rows.Err()
}

// MarkOffline marks clients as offline if they haven't been seen recently
func (s *SQLiteStore) MarkOffline(timeout time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-timeout)
	query := `UPDATE clients SET status = 'offline', updated_at = CURRENT_TIMESTAMP 
	          WHERE last_seen < ? AND status = 'online'`

	_, err := s.db.Exec(query, cutoff)
	return err
}

// DeleteClient removes a client and its proxies from the database
func (s *SQLiteStore) DeleteClient(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}

	if _, err := tx.Exec("DELETE FROM proxies WHERE client_id = ?", id); err != nil {
		tx.Rollback()
		return err
	}

	if _, err := tx.Exec("DELETE FROM clients WHERE id = ?", id); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

// UpdateClientAlias updates the alias for a client
func (s *SQLiteStore) UpdateClientAlias(clientID, alias string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(
		"UPDATE clients SET alias = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		alias,
		clientID,
	)
	return err
}

// GetStats returns statistics about stored clients
func (s *SQLiteStore) GetStats() (total, online, offline int, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	err = s.db.QueryRow("SELECT COUNT(*) FROM clients").Scan(&total)
	if err != nil {
		return
	}

	err = s.db.QueryRow("SELECT COUNT(*) FROM clients WHERE status = 'online'").Scan(&online)
	if err != nil {
		return
	}

	err = s.db.QueryRow("SELECT COUNT(*) FROM clients WHERE status = 'offline'").Scan(&offline)
	return
}

// SaveProxy saves a proxy connection to the database
func (s *SQLiteStore) SaveProxy(proxy *ProxyConnection) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
	INSERT INTO proxies (id, client_id, local_port, remote_host, remote_port, protocol, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	ON CONFLICT(id) DO UPDATE SET
		local_port = excluded.local_port,
		remote_host = excluded.remote_host,
		remote_port = excluded.remote_port,
		protocol = excluded.protocol,
		updated_at = CURRENT_TIMESTAMP
	`

	_, err := s.db.Exec(query,
		proxy.ID,
		proxy.ClientID,
		proxy.LocalPort,
		proxy.RemoteHost,
		proxy.RemotePort,
		proxy.Protocol,
	)

	return err
}

// GetProxies retrieves all proxies for a client
func (s *SQLiteStore) GetProxies(clientID string) ([]*ProxyConnection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
	SELECT id, client_id, local_port, remote_host, remote_port, protocol, created_at
	FROM proxies
	WHERE client_id = ?
	ORDER BY created_at DESC
	`

	rows, err := s.db.Query(query, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var proxies []*ProxyConnection
	for rows.Next() {
		var proxy ProxyConnection
		var createdAt time.Time

		err := rows.Scan(
			&proxy.ID,
			&proxy.ClientID,
			&proxy.LocalPort,
			&proxy.RemoteHost,
			&proxy.RemotePort,
			&proxy.Protocol,
			&createdAt,
		)

		if err != nil {
			log.Printf("Error scanning proxy row: %v", err)
			continue
		}

		proxy.CreatedAt = createdAt
		proxy.LastActive = time.Now()

		proxies = append(proxies, &proxy)
	}

	return proxies, rows.Err()
}

// GetAllProxies retrieves all proxies (for non-client-specific queries)
func (s *SQLiteStore) GetAllProxies() ([]*ProxyConnection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
	SELECT id, client_id, local_port, remote_host, remote_port, protocol, created_at
	FROM proxies
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var proxies []*ProxyConnection
	for rows.Next() {
		var proxy ProxyConnection
		var createdAt time.Time

		err := rows.Scan(
			&proxy.ID,
			&proxy.ClientID,
			&proxy.LocalPort,
			&proxy.RemoteHost,
			&proxy.RemotePort,
			&proxy.Protocol,
			&createdAt,
		)

		if err != nil {
			log.Printf("Error scanning proxy row: %v", err)
			continue
		}

		proxy.CreatedAt = createdAt
		proxy.LastActive = time.Now()

		proxies = append(proxies, &proxy)
	}

	return proxies, rows.Err()
}

// DeleteProxy marks a proxy as inactive in the database
func (s *SQLiteStore) DeleteProxy(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(
		"UPDATE proxies SET status = 'inactive', updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		id,
	)
	return err
}

// UpdateProxy updates an existing proxy connection in the database
func (s *SQLiteStore) UpdateProxy(proxy *ProxyConnection) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
	UPDATE proxies
	SET local_port = ?, remote_host = ?, remote_port = ?, protocol = ?, updated_at = CURRENT_TIMESTAMP
	WHERE id = ?
	`

	_, err := s.db.Exec(query,
		proxy.LocalPort,
		proxy.RemoteHost,
		proxy.RemotePort,
		proxy.Protocol,
		proxy.ID,
	)

	return err
}

// CleanupDuplicateProxies removes old proxy records with the same client_id and local_port
func (s *SQLiteStore) CleanupDuplicateProxies(clientID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
	DELETE FROM proxies 
	WHERE (client_id, local_port, id) IN (
		SELECT client_id, local_port, id 
		FROM proxies 
		WHERE client_id = ? 
		GROUP BY client_id, local_port 
		HAVING COUNT(*) > 1
	)
	AND id NOT IN (
		SELECT MAX(id) 
		FROM proxies 
		WHERE client_id = ? 
		GROUP BY client_id, local_port
	)
	`

	_, err := s.db.Exec(query, clientID, clientID)
	return err
}

// CreateWebUser creates a new web user (password_hash should be pre-hashed)
func (s *SQLiteStore) CreateWebUser(username, passwordHash, fullName, role string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
	INSERT INTO web_users (username, password_hash, full_name, role, status)
	VALUES (?, ?, ?, ?, 'active')
	`

	_, err := s.db.Exec(query, username, passwordHash, fullName, role)
	return err
}

// GetWebUser retrieves a web user by username
func (s *SQLiteStore) GetWebUser(username string) (*WebUser, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var user WebUser
	var passwordHash string
	var lastLogin sql.NullTime

	query := `SELECT id, username, password_hash, full_name, role, status, created_at, last_login FROM web_users WHERE username = ?`
	err := s.db.QueryRow(query, username).Scan(
		&user.ID,
		&user.Username,
		&passwordHash,
		&user.FullName,
		&user.Role,
		&user.Status,
		&user.CreatedAt,
		&lastLogin,
	)

	if err != nil {
		return nil, "", err
	}

	if lastLogin.Valid {
		user.LastLogin = &lastLogin.Time
	}

	return &user, passwordHash, nil
}

// UpdateWebUserLastLogin updates the last login time for a user
func (s *SQLiteStore) UpdateWebUserLastLogin(username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(
		"UPDATE web_users SET last_login = CURRENT_TIMESTAMP WHERE username = ?",
		username,
	)
	return err
}

// GetAllWebUsers retrieves all web users
func (s *SQLiteStore) GetAllWebUsers() ([]*WebUser, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT id, username, full_name, role, status, created_at, last_login FROM web_users ORDER BY created_at DESC`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*WebUser
	for rows.Next() {
		var user WebUser
		var lastLogin sql.NullTime

		err := rows.Scan(
			&user.ID,
			&user.Username,
			&user.FullName,
			&user.Role,
			&user.Status,
			&user.CreatedAt,
			&lastLogin,
		)

		if err != nil {
			log.Printf("Error scanning web user row: %v", err)
			continue
		}

		if lastLogin.Valid {
			user.LastLogin = &lastLogin.Time
		}

		users = append(users, &user)
	}

	return users, rows.Err()
}

// DeleteWebUser removes a web user
func (s *SQLiteStore) DeleteWebUser(username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec("DELETE FROM web_users WHERE username = ?", username)
	return err
}

// UserExists checks if a user exists
func (s *SQLiteStore) UserExists(username string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM web_users WHERE username = ?", username).Scan(&count)
	return count > 0, err
}

// AdminExists checks if any admin user exists
func (s *SQLiteStore) AdminExists() (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM web_users WHERE role = ?", "admin").Scan(&count)
	return count > 0, err
}

// GetServerSetting retrieves a server setting by key
func (s *SQLiteStore) GetServerSetting(key string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var value string
	err := s.db.QueryRow("SELECT value FROM server_settings WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil // Return empty string if setting doesn't exist
	}
	return value, err
}

// SetServerSetting saves or updates a server setting
func (s *SQLiteStore) SetServerSetting(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
	INSERT INTO server_settings (key, value, created_at, updated_at)
	VALUES (?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	ON CONFLICT(key) DO UPDATE SET
		value = excluded.value,
		updated_at = CURRENT_TIMESTAMP
	`

	_, err := s.db.Exec(query, key, value)
	return err
}

// GetAllServerSettings retrieves all server settings
func (s *SQLiteStore) GetAllServerSettings() (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	settings := make(map[string]string)
	rows, err := s.db.Query("SELECT key, value FROM server_settings")
	if err != nil {
		return settings, err
	}
	defer rows.Close()

	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return settings, err
		}
		settings[key] = value
	}
	return settings, rows.Err()
}

// DeleteServerSetting removes a server setting
func (s *SQLiteStore) DeleteServerSetting(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec("DELETE FROM server_settings WHERE key = ?", key)
	return err
}

// UpdateWebUser updates fullName and/or passwordHash for a web user
// Pass nil for fields that should not be updated
func (s *SQLiteStore) UpdateWebUser(username string, fullName, passwordHash *string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if fullName == nil && passwordHash == nil {
		return nil // Nothing to update
	}

	query := "UPDATE web_users SET updated_at = CURRENT_TIMESTAMP"
	args := []interface{}{}

	if fullName != nil {
		query += ", full_name = ?"
		args = append(args, *fullName)
	}

	if passwordHash != nil {
		query += ", password_hash = ?"
		args = append(args, *passwordHash)
	}

	query += " WHERE username = ?"
	args = append(args, username)

	_, err := s.db.Exec(query, args...)
	return err
}

// Close closes the database connection
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
