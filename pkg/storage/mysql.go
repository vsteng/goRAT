package storage

import (
	"database/sql"
	"errors"
	"time"

	"gorat/pkg/protocol"

	_ "github.com/go-sql-driver/mysql"
)

// myCfg carries minimal MySQL configuration (use Database.Path as DSN)
type myCfg struct {
	Type string
	DSN  string
}

// MySQLStore implements Store interface using MySQL backend (minimal stub)
type MySQLStore struct {
	db *sql.DB
}

// NewMySQLStore creates a new MySQL-backed store
func NewMySQLStore(cfg myCfg) (Store, error) {
	db, err := sql.Open("mysql", cfg.DSN)
	if err != nil {
		return nil, err
	}
	s := &MySQLStore{db: db}
	if err := s.initDB(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// -- Minimal implementations to satisfy Store --

func (s *MySQLStore) SaveClient(metadata *protocol.ClientMetadata) error {
	_, err := s.db.Exec(`
		INSERT INTO clients (
			id, token, os, arch, hostname, alias, ip, public_ip, status, version,
			connected_at, last_seen, last_heartbeat
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			token=VALUES(token), os=VALUES(os), arch=VALUES(arch), hostname=VALUES(hostname),
			alias=VALUES(alias), ip=VALUES(ip), public_ip=VALUES(public_ip), status=VALUES(status),
			version=VALUES(version), last_seen=VALUES(last_seen), last_heartbeat=VALUES(last_heartbeat)
	`,
		metadata.ID, metadata.Token, metadata.OS, metadata.Arch, metadata.Hostname, metadata.Alias,
		metadata.IP, metadata.PublicIP, metadata.Status, metadata.Version,
		metadata.ConnectedAt, metadata.LastSeen, metadata.LastHeartbeat,
	)
	return err
}
func (s *MySQLStore) GetClient(id string) (*protocol.ClientMetadata, error) {
	row := s.db.QueryRow(`
		SELECT id, token, os, arch, hostname, alias, ip, public_ip, status, version,
			   connected_at, last_seen, last_heartbeat
		FROM clients WHERE id = ? LIMIT 1`, id)
	var m protocol.ClientMetadata
	err := row.Scan(&m.ID, &m.Token, &m.OS, &m.Arch, &m.Hostname, &m.Alias, &m.IP, &m.PublicIP, &m.Status, &m.Version,
		&m.ConnectedAt, &m.LastSeen, &m.LastHeartbeat)
	if err != nil {
		return nil, err
	}
	return &m, nil
}
func (s *MySQLStore) GetAllClients() ([]*protocol.ClientMetadata, error) {
	rows, err := s.db.Query(`
		SELECT id, token, os, arch, hostname, alias, ip, public_ip, status, version,
			   connected_at, last_seen, last_heartbeat
		FROM clients ORDER BY connected_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*protocol.ClientMetadata
	for rows.Next() {
		var m protocol.ClientMetadata
		if err := rows.Scan(&m.ID, &m.Token, &m.OS, &m.Arch, &m.Hostname, &m.Alias, &m.IP, &m.PublicIP, &m.Status, &m.Version,
			&m.ConnectedAt, &m.LastSeen, &m.LastHeartbeat); err != nil {
			return nil, err
		}
		list = append(list, &m)
	}
	return list, rows.Err()
}
func (s *MySQLStore) MarkOffline(timeout time.Duration) error {
	// Mark clients offline if last_seen older than timeout seconds
	_, err := s.db.Exec(`
		UPDATE clients SET status='offline'
		WHERE last_seen IS NOT NULL AND TIMESTAMPDIFF(SECOND, last_seen, NOW()) > ?`, int(timeout.Seconds()))
	return err
}
func (s *MySQLStore) DeleteClient(id string) error {
	// Delete client and associated proxies
	_, err := s.db.Exec(`DELETE FROM proxies WHERE client_id = ?`, id)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`DELETE FROM clients WHERE id = ?`, id)
	return err
}
func (s *MySQLStore) UpdateClientAlias(clientID, alias string) error {
	_, err := s.db.Exec(`UPDATE clients SET alias = ?, last_seen = NOW() WHERE id = ?`, alias, clientID)
	return err
}
func (s *MySQLStore) GetStats() (int, int, int, error) {
	var total, online, offline int
	if err := s.db.QueryRow(`SELECT COUNT(1) FROM clients`).Scan(&total); err != nil {
		return 0, 0, 0, err
	}
	if err := s.db.QueryRow(`SELECT COUNT(1) FROM clients WHERE status = 'online'`).Scan(&online); err != nil {
		return 0, 0, 0, err
	}
	if err := s.db.QueryRow(`SELECT COUNT(1) FROM clients WHERE status = 'offline'`).Scan(&offline); err != nil {
		return 0, 0, 0, err
	}
	return total, online, offline, nil
}

func (s *MySQLStore) SaveProxy(proxy *ProxyConnection) error {
	_, err := s.db.Exec(`
		INSERT INTO proxies (
			id, client_id, local_port, remote_host, remote_port, protocol,
			bytes_in, bytes_out, created_at, last_active, user_count
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE 
			client_id=VALUES(client_id), local_port=VALUES(local_port), remote_host=VALUES(remote_host),
			remote_port=VALUES(remote_port), protocol=VALUES(protocol), bytes_in=VALUES(bytes_in),
			bytes_out=VALUES(bytes_out), last_active=VALUES(last_active), user_count=VALUES(user_count)
	`,
		proxy.ID, proxy.ClientID, proxy.LocalPort, proxy.RemoteHost, proxy.RemotePort, proxy.Protocol,
		proxy.BytesIn, proxy.BytesOut, proxy.CreatedAt, proxy.LastActive, proxy.UserCount,
	)
	return err
}
func (s *MySQLStore) GetProxies(clientID string) ([]*ProxyConnection, error) {
	rows, err := s.db.Query(`
		SELECT id, client_id, local_port, remote_host, remote_port, protocol,
			   bytes_in, bytes_out, created_at, last_active, user_count
		FROM proxies WHERE client_id = ? ORDER BY created_at DESC`, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*ProxyConnection
	for rows.Next() {
		var p ProxyConnection
		if err := rows.Scan(&p.ID, &p.ClientID, &p.LocalPort, &p.RemoteHost, &p.RemotePort, &p.Protocol,
			&p.BytesIn, &p.BytesOut, &p.CreatedAt, &p.LastActive, &p.UserCount); err != nil {
			return nil, err
		}
		list = append(list, &p)
	}
	return list, rows.Err()
}
func (s *MySQLStore) GetAllProxies() ([]*ProxyConnection, error) {
	rows, err := s.db.Query(`
		SELECT id, client_id, local_port, remote_host, remote_port, protocol,
			   bytes_in, bytes_out, created_at, last_active, user_count
		FROM proxies ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*ProxyConnection
	for rows.Next() {
		var p ProxyConnection
		if err := rows.Scan(&p.ID, &p.ClientID, &p.LocalPort, &p.RemoteHost, &p.RemotePort, &p.Protocol,
			&p.BytesIn, &p.BytesOut, &p.CreatedAt, &p.LastActive, &p.UserCount); err != nil {
			return nil, err
		}
		list = append(list, &p)
	}
	return list, rows.Err()
}
func (s *MySQLStore) DeleteProxy(id string) error {
	_, err := s.db.Exec(`DELETE FROM proxies WHERE id = ?`, id)
	return err
}
func (s *MySQLStore) UpdateProxy(proxy *ProxyConnection) error {
	_, err := s.db.Exec(`
		UPDATE proxies SET 
			client_id = ?, local_port = ?, remote_host = ?, remote_port = ?, protocol = ?,
			bytes_in = ?, bytes_out = ?, last_active = ?, user_count = ?
		WHERE id = ?
	`,
		proxy.ClientID, proxy.LocalPort, proxy.RemoteHost, proxy.RemotePort, proxy.Protocol,
		proxy.BytesIn, proxy.BytesOut, proxy.LastActive, proxy.UserCount, proxy.ID,
	)
	return err
}
func (s *MySQLStore) CleanupDuplicateProxies(clientID string) error {
	// Remove older duplicates for same (client_id, local_port, remote_host, remote_port, protocol)
	_, err := s.db.Exec(`
		DELETE p1 FROM proxies p1
		JOIN proxies p2
		  ON p1.client_id = p2.client_id
		 AND p1.local_port = p2.local_port
		 AND p1.remote_host = p2.remote_host
		 AND p1.remote_port = p2.remote_port
		 AND p1.protocol = p2.protocol
		 AND p1.created_at < p2.created_at
		WHERE p1.client_id = ?
	`, clientID)
	return err
}

func (s *MySQLStore) CreateWebUser(username, passwordHash, fullName, role string) error {
	_, err := s.db.Exec(`
        INSERT INTO web_users (username, password_hash, full_name, role, status, created_at, updated_at)
        VALUES (?, ?, ?, ?, 'active', NOW(), NOW())
        ON DUPLICATE KEY UPDATE updated_at = NOW()`,
		username, passwordHash, fullName, role,
	)
	return err
}

func (s *MySQLStore) GetWebUser(username string) (*WebUser, string, error) {
	row := s.db.QueryRow(`
        SELECT id, username, password_hash, full_name, role, status, created_at, updated_at, last_login
        FROM web_users WHERE username = ? LIMIT 1`, username)
	var u WebUser
	var pwd string
	var updatedAt time.Time
	err := row.Scan(&u.ID, &u.Username, &pwd, &u.FullName, &u.Role, &u.Status, &u.CreatedAt, &updatedAt, &u.LastLogin)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, "", err
		}
		return nil, "", err
	}
	return &u, pwd, nil
}

func (s *MySQLStore) UpdateWebUserLastLogin(username string) error {
	_, err := s.db.Exec(`UPDATE web_users SET last_login = NOW(), updated_at = NOW() WHERE username = ?`, username)
	return err
}

func (s *MySQLStore) GetAllWebUsers() ([]*WebUser, error) {
	rows, err := s.db.Query(`
        SELECT id, username, full_name, role, status, created_at, last_login
        FROM web_users ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*WebUser
	for rows.Next() {
		var u WebUser
		err := rows.Scan(&u.ID, &u.Username, &u.FullName, &u.Role, &u.Status, &u.CreatedAt, &u.LastLogin)
		if err != nil {
			return nil, err
		}
		list = append(list, &u)
	}
	return list, rows.Err()
}

func (s *MySQLStore) DeleteWebUser(username string) error {
	_, err := s.db.Exec(`DELETE FROM web_users WHERE username = ?`, username)
	return err
}

func (s *MySQLStore) UserExists(username string) (bool, error) {
	var cnt int
	err := s.db.QueryRow(`SELECT COUNT(1) FROM web_users WHERE username = ?`, username).Scan(&cnt)
	return cnt > 0, err
}

func (s *MySQLStore) AdminExists() (bool, error) {
	var cnt int
	err := s.db.QueryRow(`SELECT COUNT(1) FROM web_users WHERE role = 'admin'`).Scan(&cnt)
	return cnt > 0, err
}

func (s *MySQLStore) UpdateWebUser(username string, fullName, passwordHash *string) error {
	// Build dynamic update
	query := "UPDATE web_users SET updated_at = NOW()"
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

func (s *MySQLStore) UpdateWebUserStatus(username, status string) error {
	if status != "active" && status != "inactive" {
		return errors.New("invalid status")
	}
	_, err := s.db.Exec(`UPDATE web_users SET status = ?, updated_at = NOW() WHERE username = ?`, status, username)
	return err
}

func (s *MySQLStore) GetServerSetting(key string) (string, error) {
	return "", errors.New("not implemented")
}
func (s *MySQLStore) SetServerSetting(key, value string) error { return errors.New("not implemented") }
func (s *MySQLStore) GetAllServerSettings() (map[string]string, error) {
	return nil, errors.New("not implemented")
}
func (s *MySQLStore) DeleteServerSetting(key string) error { return errors.New("not implemented") }

func (s *MySQLStore) Close() error { return s.db.Close() }

// initDB creates required tables if not present
func (s *MySQLStore) initDB() error {
	schema := `
CREATE TABLE IF NOT EXISTS web_users (
    id INT AUTO_INCREMENT PRIMARY KEY,
    username VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    full_name VARCHAR(255),
    role VARCHAR(50) DEFAULT 'user',
    status VARCHAR(50) DEFAULT 'active',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    last_login DATETIME NULL
);

CREATE TABLE IF NOT EXISTS clients (
	id VARCHAR(255) PRIMARY KEY,
	token VARCHAR(255) NOT NULL,
	os VARCHAR(50) NOT NULL,
	arch VARCHAR(50) NOT NULL,
	hostname VARCHAR(255) NOT NULL,
	alias VARCHAR(255),
	ip VARCHAR(255),
	public_ip VARCHAR(255),
	status VARCHAR(50) DEFAULT 'offline',
	version VARCHAR(50),
	connected_at DATETIME,
	last_seen DATETIME,
	last_heartbeat DATETIME,
	INDEX idx_clients_status (status),
	INDEX idx_clients_last_seen (last_seen)
);

CREATE TABLE IF NOT EXISTS proxies (
	id VARCHAR(255) PRIMARY KEY,
	client_id VARCHAR(255) NOT NULL,
	local_port INT NOT NULL,
	remote_host VARCHAR(255) NOT NULL,
	remote_port INT NOT NULL,
	protocol VARCHAR(20) NOT NULL,
	bytes_in BIGINT DEFAULT 0,
	bytes_out BIGINT DEFAULT 0,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	last_active DATETIME,
	user_count INT DEFAULT 0,
	INDEX idx_proxies_client (client_id),
	INDEX idx_proxies_last_active (last_active)
);
`
	_, err := s.db.Exec(schema)
	return err
}
