package server

import (
	"database/sql"
	"encoding/json"
	"log"
	"sync"
	"time"

	"mww2.com/server_manager/common"

	_ "github.com/mattn/go-sqlite3"
)

// ClientStore manages persistent storage of client information
type ClientStore struct {
	db *sql.DB
	mu sync.RWMutex
}

// NewClientStore creates a new client store with SQLite backend
func NewClientStore(dbPath string) (*ClientStore, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	store := &ClientStore{
		db: db,
	}

	if err := store.initDB(); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

// initDB initializes the database schema
func (s *ClientStore) initDB() error {
	schema := `
	CREATE TABLE IF NOT EXISTS clients (
		id TEXT PRIMARY KEY,
		hostname TEXT,
		os TEXT,
		arch TEXT,
		ip TEXT,
		public_ip TEXT,
		status TEXT,
		last_seen DATETIME,
		first_seen DATETIME,
		metadata TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_last_seen ON clients(last_seen DESC);
	CREATE INDEX IF NOT EXISTS idx_status ON clients(status);
	`

	_, err := s.db.Exec(schema)
	return err
}

// SaveClient saves or updates a client in the database
func (s *ClientStore) SaveClient(metadata *common.ClientMetadata) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Serialize metadata to JSON for flexible storage
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

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
		metadataJSON,
	)

	return err
}

// GetClient retrieves a client by ID
func (s *ClientStore) GetClient(id string) (*common.ClientMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var metadata common.ClientMetadata
	var metadataJSON string

	query := `SELECT id, hostname, os, arch, ip, public_ip, status, last_seen, metadata FROM clients WHERE id = ?`
	err := s.db.QueryRow(query, id).Scan(
		&metadata.ID,
		&metadata.Hostname,
		&metadata.OS,
		&metadata.Arch,
		&metadata.IP,
		&metadata.PublicIP,
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
func (s *ClientStore) GetAllClients() ([]*common.ClientMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT id, hostname, os, arch, ip, public_ip, status, last_seen, metadata 
	          FROM clients 
	          ORDER BY last_seen DESC`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
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
func (s *ClientStore) MarkOffline(timeout time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-timeout)
	query := `UPDATE clients SET status = 'offline', updated_at = CURRENT_TIMESTAMP 
	          WHERE last_seen < ? AND status = 'online'`

	_, err := s.db.Exec(query, cutoff)
	return err
}

// DeleteClient removes a client from the database
func (s *ClientStore) DeleteClient(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec("DELETE FROM clients WHERE id = ?", id)
	return err
}

// Close closes the database connection
func (s *ClientStore) Close() error {
	return s.db.Close()
}

// GetStats returns statistics about stored clients
func (s *ClientStore) GetStats() (total, online, offline int, err error) {
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
