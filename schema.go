package main

import (
	"fmt"
	"strings"

	"github.com/dolthub/go-mysql-server/sql"
	"github.com/dolthub/go-mysql-server/sql/types"
	"github.com/vmihailenco/msgpack/v5"
)

const SchemaVersion = 1

const InfoProcedure = "get_info"
const SchemaProcedure = "get_schema"
const SanityCheckProcedure = "sanity_check"

type Schema struct {
	SchemaVersion int     `msgpack:"schema_version"`
	Version       int     `msgpack:"version"`
	Database      string  `msgpack:"database"`
	Users         []User  `msgpack:"users"`
	Tables        []Table `msgpack:"tables"`
}

type User struct {
	Username string `msgpack:"username"`
	Password string `msgpack:"password"`
}

type Column struct {
	Name     string `msgpack:"name"`
	Type     string `msgpack:"type"`
	Primary  bool   `msgpack:"primary"`
	Nullable bool   `msgpack:"nullable"`
	Unsigned bool   `msgpack:"unsigned"`
	Comment  string `msgpack:"comment"`
}

type Select struct {
	All    string `msgpack:"all"`
	Filter string `msgpack:"filter"`
	Count  string `msgpack:"count"`
}

type Table struct {
	Name    string   `msgpack:"name"`
	Scope   string   `msgpack:"scope"`
	Columns []Column `msgpack:"columns"`
	Select  Select   `msgpack:"select"`
	Comment string   `msgpack:"comment"`
}

func mapColumnType(t string) (sql.Type, error) {
	switch strings.ToLower(t) {
	case "int", "integer":
		return types.Int64, nil
	case "uint":
		return types.Uint64, nil
	case "float":
		return types.Float64, nil
	case "str", "string", "text":
		return types.Text, nil
	case "bool", "boolean":
		return types.Boolean, nil
	case "datetime", "time":
		return types.Datetime, nil
	case "date":
		return types.Date, nil
	case "bytes", "bytearray":
		return types.Blob, nil
	default:
		return nil, fmt.Errorf("unsupported column type: %s", t)
	}
}

func setupCollection(schemaScope string) error {
	_, err := conn.QueryRaw(schemaScope, strings.ReplaceAll(`
		try(commit("Setup schema collection for the Mysql Endpoint module"));

		new_type('Column', HID);
		new_type('Schema', HID);
		new_type('Select', HID);
		new_type('Table', HID);
		new_type('User', HID);

		set_enum('ColumnType', {
			INT64: 'int64',
			INT32: 'int32',
			FLOAT64: 'float64',
			STRING: 'string',
			BYTES: 'bytes',
			BOOL: 'bool',
			DATETIME: 'datetime',
			TIME: 'time',
			DATE: 'date',
			JSON: 'json',
		});

		set_type('Column', {
			name: 'str',
			type: 'ColumnType',
			primary: 'bool?',
			nullable: 'bool?',
			unsigned: 'bool?',
			comment: 'str?',
		}, HID);
		set_type('Select', {
			all: 'str',
			filter: 'str?',
			count: 'str?',
		}, HID);
		set_type('Table', {
			name: 'str',
			scope: 'str',
			columns: '[Column]',
			select: 'Select',
			comment: 'str?',
		}, HID);
		set_type('User', {
			username: 'str',
			password: 'str',
		}, HID);
		set_type('Schema', {
			schema_version: 'pint(ro)',
			version: 'pint',
			database: 'str',
			users: '{User}',
			tables: '{Table}',
			ev: 'room',
		}, HID);

		.to_type('Schema');

		new_procedure('get_schema', || .wrap(&{
			schema_version: 'pint',
			version: 'pint',
			database: 'str',
			users: [{
				username: 'str',
				password: 'str',
			}],
			tables: [{
				name: 'str',
				scope: 'str',
				columns: [{
					name: 'str',
					type: 'any',
					primary: 'bool?',
					nullable: 'bool?',
					unsigned: 'bool?',
					comment: 'str?',
				}],
				select: {
					all: 'str',
					filter: 'str?',
					count: 'str?',
				},
				comment: 'str?',
			}]
		}));

		new_procedure('get_info', |module| {
			mysql_listen_port: module.mysql_listen_port
		});

		new_procedure('sanity_check', |schema_version| {
			assert(
				type(root()) == 'Schema',
				$Invalid collection root: expected type "Schema", but got "{type(root())}"$
			);
			assert(
				.get('schema_version') == schema_version,
				$Unsupported schema_version: expected {schema_version}, but found {.get('schema_version')}$
			);
			nil;
		});

		'DONE';
	`, "$", "`"), nil)
	return err
}

func ensureCollection(schemaScope string) error {
	if conn == nil {
		return fmt.Errorf("no thingsdb connection")
	}

	collectionName, err := cnScope(schemaScope)
	if err != nil {
		return err
	}

	vars := map[string]any{
		"name": collectionName,
	}

	var hasCollection bool
	res, err := conn.QueryRaw("/t", `
		has_collection(name);
	`, vars)
	if err != nil {
		return err
	}
	if err := msgpack.Unmarshal(res, &hasCollection); err != nil {
		return err
	}

	if !hasCollection {
		_, err := conn.QueryRaw("/t", `
			try(commit("Add schema collection for the Mysql Endpoint module"));
			new_collection(name);
		`, vars)
		if err != nil {
			return err
		}
		return setupCollection(schemaScope)
	}

	_, err = conn.RunRaw(schemaScope, SanityCheckProcedure, SchemaVersion)
	return err
}
