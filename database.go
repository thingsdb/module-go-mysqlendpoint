package main

import (
	"fmt"
	"sync"

	"github.com/dolthub/go-mysql-server/sql"
)

type GatewayDatabase struct {
	name   string
	mu     sync.RWMutex
	tables map[string]sql.Table
}

var _ sql.Database = (*GatewayDatabase)(nil)
var _ sql.ViewDatabase = (*GatewayDatabase)(nil)

func NewGatewayDatabase(name string) *GatewayDatabase {
	return &GatewayDatabase{
		name:   name,
		tables: make(map[string]sql.Table),
	}
}

func (db *GatewayDatabase) Name() string {
	return db.name
}

func (db *GatewayDatabase) GetTableInsensitive(ctx *sql.Context, tblName string) (sql.Table, bool, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	tbl, ok := sql.GetTableInsensitive(tblName, db.tables)
	return tbl, ok, nil
}

func (db *GatewayDatabase) GetTableNames(ctx *sql.Context) ([]string, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	names := make([]string, 0, len(db.tables))
	for name := range db.tables {
		names = append(names, name)
	}
	return names, nil
}

func (db *GatewayDatabase) ClearTables() {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.tables = make(map[string]sql.Table)
}

func (db *GatewayDatabase) AddTable(name string, tbl sql.Table) {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.tables[name] = tbl
}

// ThingsDB manages tables, return an empty list.
func (db *GatewayDatabase) AllViews(ctx *sql.Context) ([]sql.ViewDefinition, error) {
	return nil, nil
}

// GetViewDefinition retrieves a view by name. Return false since no views exist.
func (db *GatewayDatabase) GetViewDefinition(ctx *sql.Context, viewName string) (sql.ViewDefinition, bool, error) {
	return sql.ViewDefinition{}, false, nil
}

// CreateView is a no-op / unsupported operation for a read-only ThingsDB gateway.
func (db *GatewayDatabase) CreateView(ctx *sql.Context, name string, selectStatement string, textDefinition string) error {
	return fmt.Errorf("database '%s' is read-only: creating views is not supported", db.name)
}

// DropView is a no-op / unsupported operation.
func (db *GatewayDatabase) DropView(ctx *sql.Context, name string) error {
	return fmt.Errorf("database '%s' is read-only: dropping views is not supported", db.name)
}
