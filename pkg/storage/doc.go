// Package storage provides persistent data storage abstraction for goRAT.
//
// This package defines interfaces and implementations for storing client metadata,
// proxy connections, web users, and server settings. The primary implementation
// uses SQLite for reliability and simplicity.
//
// Usage:
//
//	store, err := storage.NewSQLiteStore("./clients.db")
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer store.Close()
//
//	// Save a client
//	err = store.SaveClient(&common.ClientMetadata{...})
//
//	// Retrieve all clients
//	clients, err := store.GetAllClients()
//
// The Store interface allows for alternative implementations such as PostgreSQL,
// MySQL, or other backends while maintaining API compatibility.
package storage
