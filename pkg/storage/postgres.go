package storage

import (
	"database/sql"
	"errors"
	"time"

	"gorat/pkg/protocol"
)

// PostgresStore implements Store interface using PostgreSQL backend (minimal stub)
type PostgresStore struct {
	db *sql.DB
}

type pgCfg struct {
	Type string
	Path string // use as DSN for simplicity
}

// NewPostgresStore creates a new PostgreSQL-backed store
func NewPostgresStore(cfg pgCfg) (Store, error) {
	db, err := sql.Open("postgres", cfg.Path)
	if err != nil {
		return nil, err
	}
	return &PostgresStore{db: db}, nil
}

// -- Minimal implementations to satisfy Store --

func (s *PostgresStore) SaveClient(metadata *protocol.ClientMetadata) error {
	return errors.New("not implemented")
}
func (s *PostgresStore) GetClient(id string) (*protocol.ClientMetadata, error) {
	return nil, errors.New("not implemented")
}
func (s *PostgresStore) GetAllClients() ([]*protocol.ClientMetadata, error) {
	return nil, errors.New("not implemented")
}
func (s *PostgresStore) MarkOffline(timeout time.Duration) error {
	return errors.New("not implemented")
}
func (s *PostgresStore) DeleteClient(id string) error { return errors.New("not implemented") }
func (s *PostgresStore) UpdateClientAlias(clientID, alias string) error {
	return errors.New("not implemented")
}
func (s *PostgresStore) GetStats() (int, int, int, error) {
	return 0, 0, 0, errors.New("not implemented")
}

func (s *PostgresStore) SaveProxy(proxy *ProxyConnection) error { return errors.New("not implemented") }
func (s *PostgresStore) GetProxies(clientID string) ([]*ProxyConnection, error) {
	return nil, errors.New("not implemented")
}
func (s *PostgresStore) GetAllProxies() ([]*ProxyConnection, error) {
	return nil, errors.New("not implemented")
}
func (s *PostgresStore) DeleteProxy(id string) error { return errors.New("not implemented") }
func (s *PostgresStore) UpdateProxy(proxy *ProxyConnection) error {
	return errors.New("not implemented")
}
func (s *PostgresStore) CleanupDuplicateProxies(clientID string) error {
	return errors.New("not implemented")
}

func (s *PostgresStore) CreateWebUser(username, passwordHash, fullName, role string) error {
	return errors.New("not implemented")
}
func (s *PostgresStore) GetWebUser(username string) (*WebUser, string, error) {
	return nil, "", errors.New("not implemented")
}
func (s *PostgresStore) UpdateWebUserLastLogin(username string) error {
	return errors.New("not implemented")
}
func (s *PostgresStore) GetAllWebUsers() ([]*WebUser, error) {
	return nil, errors.New("not implemented")
}
func (s *PostgresStore) DeleteWebUser(username string) error { return errors.New("not implemented") }
func (s *PostgresStore) UserExists(username string) (bool, error) {
	return false, errors.New("not implemented")
}
func (s *PostgresStore) AdminExists() (bool, error) { return false, errors.New("not implemented") }
func (s *PostgresStore) UpdateWebUser(username string, fullName, passwordHash *string) error {
	return errors.New("not implemented")
}
func (s *PostgresStore) UpdateWebUserStatus(username, status string) error {
	return errors.New("not implemented")
}

func (s *PostgresStore) GetServerSetting(key string) (string, error) {
	return "", errors.New("not implemented")
}
func (s *PostgresStore) SetServerSetting(key, value string) error {
	return errors.New("not implemented")
}
func (s *PostgresStore) GetAllServerSettings() (map[string]string, error) {
	return nil, errors.New("not implemented")
}
func (s *PostgresStore) DeleteServerSetting(key string) error { return errors.New("not implemented") }

func (s *PostgresStore) Close() error { return s.db.Close() }
