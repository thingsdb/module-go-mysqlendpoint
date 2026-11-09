package main

import (
	"context"
	"net"

	sqle "github.com/dolthub/go-mysql-server"
	"github.com/dolthub/go-mysql-server/server"
	"github.com/dolthub/go-mysql-server/sql"
	"github.com/dolthub/vitess/go/mysql"
)

func newDefaultServer(cfg server.Config, engine *sqle.Engine) (*server.Server, error) {
	sessionBuilder := func(ctx context.Context, conn *mysql.Conn, addr string) (sql.Session, error) {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			host = addr
		}
		client := sql.Client{
			Address: host,
			User:    conn.User,
		}
		return sql.NewBaseSessionWithClientServer(cfg.Address, client, conn.ConnectionID), nil
	}

	var ctxFactory sql.ContextFactory = sql.NewContext

	return server.NewServer(cfg, engine, ctxFactory, sessionBuilder, nil)
}
