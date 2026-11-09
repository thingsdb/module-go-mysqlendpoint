package main

import (
	"log"

	thingsdb "github.com/thingsdb/go-thingsdb"
)

const ReloadEvent = "reload"

func onReload(room *thingsdb.Room, args []any) {
	err := gateway.Reload(conn, *cfg.SchemaScope)
	if err != nil {
		log.Printf("reload failed: %v\n", err)
	}
}
