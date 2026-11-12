package main

import (
	"fmt"
	"log"
	"strings"
	"sync"

	sqle "github.com/dolthub/go-mysql-server"
	"github.com/dolthub/go-mysql-server/sql"
	"github.com/dolthub/go-mysql-server/sql/mysql_db"

	thingsdb "github.com/thingsdb/go-thingsdb"
)

type GatewayState struct {
	mu            sync.RWMutex
	currentSchema *Schema
	dbProvider    *GatewayDatabaseProvider
	mysqlDb       *mysql_db.MySQLDb
	currentDb     *GatewayDatabase
	engine        *sqle.Engine
	listenAddr    string
}

func newGateway() *GatewayState {
	vars := make(map[string]interface{})
	vars["secure_file_priv"] = "NULL"
	if err := sql.SystemVariables.AssignValues(vars); err != nil {
		log.Printf("failed to set secure_file_priv: %v\n", err)
	}

	provider := NewGatewayDatabaseProvider()

	mySQLDb := mysql_db.CreateEmptyMySQLDb()
	mySQLDb.AddRootAccount()
	mySQLDb.SetPersister(noopMySQLDbPersister{})

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

	if g.currentDb == nil || g.currentDb.Name() != schema.Database {
		if g.currentDb != nil {
			g.dbProvider.DropDatabase(g.currentDb.Name())
		}
		newDb := NewGatewayDatabase(schema.Database)
		g.dbProvider.AddDatabase(newDb)
		g.currentDb = newDb
	}

	g.syncTables(&schema, tiConn)
	g.syncUsers(&schema)

	g.currentSchema = &schema
	log.Printf("Gateway reloaded: db=%s, tables=%d, users=%d (v%d)",
		schema.Database, len(schema.Tables), len(schema.Users), schema.Version)

	return nil
}

func (g *GatewayState) syncTables(s *Schema, tiConn *thingsdb.Conn) {
	if g.currentDb == nil {
		return
	}

	g.currentDb.ClearTables()

	for _, tblDef := range s.Tables {
		liveTbl, err := NewLiveTable(g.currentDb.Name(), tblDef, tiConn)
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
	ctx.SetClient(sql.Client{
		User:    "root",
		Address: "localhost",
	})

	for _, u := range s.Users {
		user := escapeSQL(u.Username)
		pass := escapeSQL(u.Password)

		createQuery := fmt.Sprintf(
			"CREATE USER IF NOT EXISTS '%s'@'%%' IDENTIFIED WITH mysql_native_password BY '%s';",
			user, pass,
		)
		if _, _, _, err := g.engine.Query(ctx, createQuery); err != nil {
			log.Printf("error ensuring user %s: %v", user, err)
		}

		alterQuery := fmt.Sprintf(
			"ALTER USER '%s'@'%%' IDENTIFIED WITH mysql_native_password BY '%s';",
			user, pass,
		)
		if _, _, _, err := g.engine.Query(ctx, alterQuery); err != nil {
			log.Printf("error updating credentials for user %s: %v", user, err)
		}

		grantQuery := fmt.Sprintf(
			"GRANT SELECT ON `%s`.* TO '%s'@'%%';",
			escapeSQL(s.Database), user,
		)
		if _, _, _, err := g.engine.Query(ctx, grantQuery); err != nil {
			log.Printf("error updating privileges for user %s: %v", user, err)
		}
	}
}

func escapeSQL(val string) string {
	return strings.ReplaceAll(val, "'", "''")
}
