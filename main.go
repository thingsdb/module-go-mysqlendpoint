// ThingsDB module for communication with ThingsDB.
//
// For example:
//
//		// Create the module (@thingsdb scope)
//		new_module('thingsdb', 'github.com/thingsdb/module-go-thingsdb');
//
//		// Configure the module
//		set_module_conf('thingsdb', {
//		    username: 'admin',
//		    password: 'pass',
//	     host: 'localhost',
//	     default_scope: '//stuff',
//		});
//
//		// Use the module
//		thingsdb.query(".id()").then(|res| {
//		    res;  // just return the response.
//		});
package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/dolthub/go-mysql-server/server"
	thingsdb "github.com/thingsdb/go-thingsdb"
	timod "github.com/thingsdb/go-timod"
	"github.com/vmihailenco/msgpack/v5"
)

var (
	conn      *thingsdb.Conn
	room      *thingsdb.Room
	cfg       config
	mux       sync.RWMutex
	NilBytes  = []byte{0xc0}
	gateway   *GatewayState
	srv       *server.Server
	srvCancel context.CancelFunc
)

func startOrRestartServer(bindAddr string, port int) error {
	targetAddr := fmt.Sprintf("%s:%d", bindAddr, port)

	if srv != nil && gateway.listenAddr == targetAddr {
		// Address didn't change; keep existing connections intact
		return nil
	}

	// Shut down existing listener if running
	if srv != nil {
		log.Printf("Restarting MySQL listener from %s to %s", gateway.listenAddr, targetAddr)
		if srvCancel != nil {
			srvCancel()
		}
		_ = srv.Close()
		srv = nil
	}

	var err error
	srv, err = newDefaultServer(server.Config{
		Protocol: "tcp",
		Address:  targetAddr,
	}, gateway.engine)
	if err != nil {
		return err
	}

	gateway.listenAddr = targetAddr

	ctx, cancel := context.WithCancel(context.Background())
	srvCancel = cancel

	go func() {
		log.Printf("MySQL endpoint listening on %s", targetAddr)
		if err := srv.Start(); err != nil && ctx.Err() == nil {
			log.Printf("MySQL server terminated: %v", err)
		}
	}()

	return nil
}

type config struct {
	ThingsdbToken    *string `msgpack:"thingsdb_token"`
	ThingsdbPort     *uint16 `msgpack:"thingsdb_port"`
	ThingsdbTimeout  *uint16 `msgpack:"thingsdb_timeout"`
	SchemaScope      *string `msgpack:"schema_scope"`
	MysqlBindAddress *string `msgpack:"mysql_bind_address"`
	MysqlListenPort  *uint16 `msgpack:"mysql_listen_port"`
}

type request struct {
	Method *string `msgpack:"method"`
}

func ensureConfig() error {
	if cfg.ThingsdbToken == nil {
		return fmt.Errorf("missing thingsdb_token")
	}

	if cfg.ThingsdbPort == nil {
		*cfg.ThingsdbPort = 9200
	}

	if cfg.ThingsdbTimeout == nil {
		*cfg.ThingsdbTimeout = 10
	}

	if cfg.SchemaScope == nil {
		*cfg.SchemaScope = "//schema"
	}

	if cfg.MysqlBindAddress == nil {
		*cfg.MysqlBindAddress = "0.0.0.0"
	}

	if cfg.MysqlListenPort == nil {
		*cfg.MysqlListenPort = 3306
	}

	return nil
}

func onConfError(err error) {
	if conn != nil {
		conn.Close()
		conn = nil
	}
	if room != nil {
		_ = room.Leave()
		room = nil
	}
	log.Println(err)
	timod.WriteConfErr()
}

func handleConf() {
	mux.Lock()
	defer mux.Unlock()

	if err := ensureConfig(); err != nil {
		log.Println(err)
		timod.WriteConfErr()
		return
	}

	if room != nil {
		_ = room.Leave()
		room = nil
	}

	if conn != nil {
		conn.Close()
	}

	conn = thingsdb.NewConn("localhost", *cfg.ThingsdbPort, nil)
	conn.DefaultTimeout = time.Duration(*cfg.ThingsdbTimeout) * time.Second

	if err := conn.Connect(); err != nil {
		onConfError(err)
		return
	}

	if err := conn.AuthToken(*cfg.ThingsdbToken); err != nil {
		onConfError(err)
		return
	}

	if err := ensureCollection(*cfg.SchemaScope); err != nil {
		onConfError(err)
		return
	}

	room = thingsdb.NewRoom(*cfg.SchemaScope, ".ev.id();")
	room.HandleEvent(ReloadEvent, onReload)

	if err := room.Join(conn, thingsdb.DefaultWait); err != nil {
		onConfError(err)
		return
	}

	// at this point, we have a schema collection but it may be empty (no database name set, and/or no users or tables, but it might be that all is configuresd)
	// here, we want to set-up the "sql" server gateway:
	// 	type GatewayState struct {
	// 	mu            sync.RWMutex
	// 	currentSchema *Schema
	// 	dbProvider    *memory.DbProvider
	// 	mysqlDb       *mysql_db.MySQLDb
	// 	currentDb     *memory.Database
	// }
	// should we best keep a global instance? and create it here?
	// init can be be: if nil -> create. try "reload", reload could work on init as well?

	if err := startOrRestartServer(*cfg.MysqlBindAddress, int(*cfg.MysqlListenPort)); err != nil {
		onConfError(err)
	}

	timod.WriteConfOk()
}

func onResponse(pkg *timod.Pkg, res []byte, err error) {
	if err != nil {
		if terr, ok := err.(*thingsdb.TiError); ok {
			timod.WriteEx(
				pkg.Pid,
				timod.Ex(terr.Code()),
				terr.Error())
			return
		}
		timod.WriteEx(
			pkg.Pid,
			timod.ExBadData,
			fmt.Sprintf("Unexpected error: %s", err))
		return
	}
	timod.WriteResponseRaw(pkg.Pid, res)
}

func onModuleReq(pkg *timod.Pkg) {
	mux.RLock()
	defer mux.RUnlock()

	if conn == nil {
		timod.WriteEx(
			pkg.Pid,
			timod.ExCancelled,
			"No connection, make sure you used set_module_conf() to initialze the module")
		return
	}

	if room == nil {
		timod.WriteEx(
			pkg.Pid,
			timod.ExCancelled,
			"Room not joined, make sure you used set_module_conf() to initialze the module")
		return
	}

	if err := ensureConfig(); err != nil {
		timod.WriteEx(
			pkg.Pid,
			timod.ExCancelled,
			err.Error())
		return
	}

	var req request
	if err := msgpack.Unmarshal(pkg.Data, &req); err == nil {
		timod.WriteEx(
			pkg.Pid,
			timod.ExCancelled,
			"Invalid method. Expection 'info' or 'reload'.")
		return
	}

	if *req.Method == "info" {
		vars := map[string]any{
			"name": "test",
		}
		res, err := conn.RunRaw(*cfg.SchemaScope, InfoProcedure, vars)
		onResponse(pkg, res, err)
		return
	}

	if *req.Method == "reload" {
		err := room.Emit(ReloadEvent, nil)
		onResponse(pkg, NilBytes, err)
		return
	}

	timod.WriteEx(
		pkg.Pid,
		timod.ExCancelled,
		"Invalid method. Expection 'info'.")
}

func handler(buf *timod.Buffer, quit chan bool) {
	for {
		select {
		case pkg := <-buf.PkgCh:
			switch timod.Proto(pkg.Tp) {
			case timod.ProtoModuleConf:

				err := msgpack.Unmarshal(pkg.Data, &cfg)
				if err == nil {
					handleConf()
				} else {
					log.Println("Error: Missing or invalid configuration")
					timod.WriteConfErr()
				}

			case timod.ProtoModuleReq:
				onModuleReq(pkg)

			default:
				log.Printf("Error: Unexpected package type: %d", pkg.Tp)
			}
		case err := <-buf.ErrCh:
			// In case of an error you probably want to quit the module.
			// ThingsDB will try to restart the module a few times if this
			// happens.
			log.Printf("Error: %s", err)
			quit <- true
		}
	}
}

func main() {
	// Create a gateway
	gateway = newGateway()

	// Starts the module
	timod.StartModule("thingsdb", handler)

	if conn != nil {
		conn.Close()
	}
}
