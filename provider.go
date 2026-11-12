package main

import (
	"sync"

	"github.com/dolthub/go-mysql-server/sql"
)

type GatewayDatabaseProvider struct {
	mu  sync.RWMutex
	dbs map[string]sql.Database
}

var _ sql.DatabaseProvider = (*GatewayDatabaseProvider)(nil)

func NewGatewayDatabaseProvider() *GatewayDatabaseProvider {
	return &GatewayDatabaseProvider{
		dbs: make(map[string]sql.Database),
	}
}

func (p *GatewayDatabaseProvider) Database(ctx *sql.Context, name string) (sql.Database, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	db, ok := p.dbs[name]
	if !ok {
		return nil, sql.ErrDatabaseNotFound.New(name)
	}
	return db, nil
}

func (p *GatewayDatabaseProvider) HasDatabase(ctx *sql.Context, name string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	_, ok := p.dbs[name]
	return ok
}

func (p *GatewayDatabaseProvider) AllDatabases(ctx *sql.Context) []sql.Database {
	p.mu.RLock()
	defer p.mu.RUnlock()
	all := make([]sql.Database, 0, len(p.dbs))
	for _, db := range p.dbs {
		all = append(all, db)
	}
	return all
}

func (p *GatewayDatabaseProvider) AddDatabase(db sql.Database) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.dbs[db.Name()] = db
}

func (p *GatewayDatabaseProvider) DropDatabase(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.dbs, name)
}
