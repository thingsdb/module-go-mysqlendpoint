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

type ForeignKey struct {
	Table  string `json:"table"`
	Column string `json:"column"`
}

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
	Scope    string `msgpack:"scope"`
}

type Column struct {
	Name       string      `msgpack:"name"`
	Type       string      `msgpack:"type"`
	Primary    bool        `msgpack:"primary"`
	Nullable   bool        `msgpack:"nullable"`
	Comment    string      `msgpack:"comment"`
	ForeignKey *ForeignKey `msgpack:"foreign_key"`
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
	switch t {
	case "Int64":
		return types.Int64, nil
	case "Int32":
		return types.Int32, nil
	case "Int16":
		return types.Int16, nil
	case "Int8":
		return types.Int8, nil
	case "Uint64":
		return types.Uint64, nil
	case "Uint32":
		return types.Uint32, nil
	case "Uint16":
		return types.Uint16, nil
	case "Uint8":
		return types.Uint8, nil
	case "Float64":
		return types.Float64, nil
	case "Float32":
		return types.Float32, nil
	case "Boolean":
		return types.Boolean, nil
	case "TinyText":
		return types.TinyText, nil
	case "Text":
		return types.Text, nil
	case "LongText":
		return types.LongText, nil
	case "Blob":
		return types.Blob, nil
	case "Datetime":
		return types.Datetime, nil
	case "Timestamp":
		return types.Timestamp, nil
	case "JSON":
		return types.JSON, nil

	default:
		return nil, fmt.Errorf("unsupported column type: %s", t)
	}
}

func setupCollection(schemaScope string) error {
	_, err := conn.QueryRaw(schemaScope, strings.ReplaceAll(`
		try(commit("Setup schema collection for the Mysql Endpoint module"));

		new_type('ForeignKey', HID);
		new_type('Column', HID);
		new_type('Schema', HID);
		new_type('Select', HID);
		new_type('Table', HID);
		new_type('User', HID);
		new_type('Info', HID);

		set_type('Info', {
			config: '&any',
			schema: '&any',
		});

		set_enum('ColumnType', {
			INT64: 'Int64',
			INT32: 'Int32',
			INT16: 'Int16',
			INT8: 'Int8',
			UINT64: 'Uint64',
			UINT32: 'Uint32',
			UINT16: 'Uint16',
			UINT8: 'Uint8',
			FLOAT64: 'Float64',
			FLOAT32: 'Float32',
			BOOLEAN: 'Boolean',
			TINYTEXT: 'TinyText',
			TEXT: 'Text',
			LONGTEXT: 'LongText',
			BLOB: 'Blob',
			DATETIME: 'Datetime',
			TIMESTAMP: 'Timestamp',
			JSON: 'JSON',
		});
		set_type('ForeignKey', {
			table: 'str',
			column: 'str',
		});
		set_type('Column', {
			name: 'str',
			type: 'ColumnType',
			primary: 'bool',
			nullable: 'bool',
			foreign_key: 'ForeignKey?',
			comment: 'str?',
		});
		set_type('Select', {
			all: 'str',
			filter: 'str?',
			count: 'str?',
		});
		set_type('Table', {
			name: 'str',
			scope: 'str?',  // if nil, scope must be defined by user.scope
			columns: '[Column]',
			select: 'Select',
			comment: 'str?',
		});
		set_type('User', {
			username: 'str',
			password: 'str',
			scope: 'str?',  // if set, the table.scope will be ignore
		});
		set_type('Schema', {
			schema_version: 'pint(ro)',
			version: 'pint',
			database: 'str',
			users: '{User}',
			tables: '{Table}',
			ev: 'room',
		});

		.to_type('Schema');

		new_procedure('get_schema', || .wrap(&{
			schema_version: 'pint',
			version: 'pint',
			database: 'str',
			users: [{
				username: 'str',
				password: 'str',
				scope: 'str?',
			}],
			tables: [{
				name: 'str',
				scope: 'str?',
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

		new_procedure('get_info', |cfg| Info{config: cfg, schema: get_schema()}.wrap());

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

	_, err = conn.RunRaw(schemaScope, SanityCheckProcedure, []any{SchemaVersion})
	return err
}
