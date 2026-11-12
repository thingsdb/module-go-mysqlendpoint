package main

import (
	"github.com/dolthub/go-mysql-server/sql"
)

type noopMySQLDbPersister struct{}

func (noopMySQLDbPersister) Persist(ctx *sql.Context, data []byte) error {
	return nil
}

func (noopMySQLDbPersister) LoadData(ctx *sql.Context) ([]byte, error) {
	return nil, nil
}
