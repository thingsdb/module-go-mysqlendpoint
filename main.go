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
	"fmt"
	"log"
	"sync"
	"time"

	thingsdb "github.com/thingsdb/go-thingsdb"
	timod "github.com/thingsdb/go-timod"
	"github.com/vmihailenco/msgpack/v5"
)

var conn *thingsdb.Conn
var cfg config
var mux sync.RWMutex

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
		return fmt.Errorf("Missing thingsdb_token")
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

func handleConf() {
	mux.Lock()
	defer mux.Unlock()

	if err := ensureConfig(); err != nil {
		log.Println(err)
		timod.WriteConfErr()
		return
	}

	if conn != nil {
		conn.Close()
	}

	conn = thingsdb.NewConn("localhost", *cfg.ThingsdbPort, nil)
	conn.DefaultTimeout = time.Duration(*cfg.ThingsdbTimeout) * time.Second

	if err := conn.Connect(); err != nil {
		conn = nil
		log.Println(err)
		timod.WriteConfErr()
		return
	}

	if err := conn.AuthToken(*cfg.ThingsdbToken); err != nil {
		conn.Close()
		conn = nil
		log.Println(err)
		timod.WriteConfErr()
		return
	}

	if err := ensureCollection(*cfg.SchemaScope); err != nil {
		conn.Close()
		conn = nil
		log.Println(err)
		timod.WriteConfErr()
		return
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
			"Invalid method. Expection 'info'.")
		return
	}

	if *req.Method == "info" {
		vars := map[string]any{
			"name": "test",
		}
		res, err := conn.RunRaw(*cfg.SchemaScope, "info", vars)
		onResponse(pkg, res, err)
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
	// Starts the module
	timod.StartModule("thingsdb", handler)

	if conn != nil {
		conn.Close()
	}
}
