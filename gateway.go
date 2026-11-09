package main

import (
	"fmt"
	"log"
	"strings"
	"sync"

	sqle "github.com/dolthub/go-mysql-server"
	"github.com/dolthub/go-mysql-server/memory"
	"github.com/dolthub/go-mysql-server/sql"
	"github.com/dolthub/go-mysql-server/sql/mysql_db"
	thingsdb "github.com/thingsdb/go-thingsdb"
)

type GatewayState struct {
	mu            sync.RWMutex
	currentSchema *Schema
	dbProvider    *memory.DbProvider
	mysqlDb       *mysql_db.MySQLDb
	currentDb     *memory.Database
	engine        *sqle.Engine
	listenAddr    string // tracks "host:port" to detect port changes
}

func newGateway() *GatewayState {
	provider := memory.NewDBProvider()

	mySQLDb := mysql_db.CreateEmptyMySQLDb()
	mySQLDb.AddRootAccount() // Enables authentication checks

	engine := sqle.NewDefault(provider)
	engine.Analyzer.Catalog.MySQLDb = mySQLDb

	return &GatewayState{
		dbProvider: provider,
		mysqlDb:    mySQLDb,
		engine:     engine,
	}
}

func (g *GatewayState) Reload(tiConn *thingsdb.Conn, scope string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	var schema Schema
	err := tiConn.RunInto(scope, SchemaProcedure, nil, &schema)
	if err != nil {
		return fmt.Errorf("failed to read schema: %w", err)
	}

	if schema.Database == "" {
		log.Println("ThingsDB schema is empty, waiting for initialization")
		return nil
	}

	if g.currentDb == nil || (*g.currentDb).Name() != schema.Database {
		if g.currentDb != nil {
			if err := g.dbProvider.DropDatabase(nil, (*g.currentDb).Name()); err != nil {
				log.Printf("drop database failed: %v\n", err)
			}
		}
		if err := g.dbProvider.CreateDatabase(nil, schema.Database); err != nil {
			return err
		}
		newDb, err := g.dbProvider.Database(nil, schema.Database)
		if err != nil {
			return err
		}
		memDb, ok := newDb.(*memory.Database)
		if !ok {
			return fmt.Errorf("unexpected database implementation: expected *memory.Database, got %T", newDb)
		}
		g.currentDb = memDb
	}

	g.syncTables(&schema)
	g.syncUsers(&schema)

	g.currentSchema = &schema
	log.Printf("Gateway reloaded: db=%s, tables=%d, users=%d (v%d)",
		schema.Database, len(schema.Tables), len(schema.Users), schema.Version)

	return nil
}
func (g *GatewayState) syncTables(s *Schema) {
	if g.currentDb == nil {
		return
	}

	ctx := sql.NewEmptyContext()

	for _, tbl := range g.currentDb.Tables() {
		if err := g.currentDb.DropTable(ctx, tbl.Name()); err != nil {
			log.Printf("warning: drop table %s: %v", tbl.Name(), err)
		}
	}

	for _, tblDef := range s.Tables {
		liveTbl, err := NewLiveTable(g.currentDb, tblDef, conn)
		if err != nil {
			log.Printf("failed to build table '%s': %v", tblDef.Name, err)
			continue
		}

		g.currentDb.AddTable(tblDef.Name, liveTbl)
	}

	log.Printf("Mounted %d live tables in database '%s'", len(s.Tables), g.currentDb.Name())
}

func (g *GatewayState) syncUsers(s *Schema) {
	ctx := sql.NewEmptyContext()

	for _, u := range s.Users {
		user := escapeSQL(u.Username)
		pass := escapeSQL(u.Password)

		createQuery := fmt.Sprintf(
			"CREATE USER IF NOT EXISTS '%s'@'%%' IDENTIFIED WITH mysql_native_password BY '%s';",
			user, pass,
		)
		_, _, _, _ = g.engine.Query(ctx, createQuery)

		alterQuery := fmt.Sprintf(
			"ALTER USER '%s'@'%%' IDENTIFIED WITH mysql_native_password BY '%s';",
			user, pass,
		)
		if _, _, _, err := g.engine.Query(ctx, alterQuery); err != nil {
			log.Printf("failed updating credentials for user %s: %v", user, err)
		}

		grantQuery := fmt.Sprintf(
			"GRANT SELECT ON `%s`.* TO '%s'@'%%';",
			escapeSQL(s.Database), user,
		)
		_, _, _, _ = g.engine.Query(ctx, grantQuery)
	}
}

func escapeSQL(val string) string {
	return strings.ReplaceAll(val, "'", "''")
}
