package main

import (
	"fmt"
	"strings"

	"github.com/vmihailenco/msgpack/v5"
)

func setupCollection(schemaScope string) error {
	_, err := conn.QueryRaw(schemaScope, `
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
		});
		set_type('Schema', {
			version: 'pint(ro)',
			database: 'str',
			users: '{User}',
			tables: '{Table}',
			trigger: 'room',
		});
		set_type('Select', {
			all: 'str',
			filter: 'str?',
			count: 'str?',
		});
		set_type('Table', {
			name: 'str',
			scope: 'str',
			columns: '[Column]',
			select: 'Select',
			comment: 'str?',
		});
		set_type('User', {
			username: 'str',
			password: 'str',
		});

		.to_type('Schema');

		'DONE';
	`, nil)
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

	_, err = conn.QueryRaw(schemaScope, strings.ReplaceAll(`
		assert(
			type(root()) == 'Schema',
			$Invalid collection root: expected type "Schema", but got "{type(root())}"$
		);

		assert(
			.get('version') == 1,
			$Unsupported schema version: expected 1, but found {.get('version')}$
		);

		nil;
	`, "$", "`"), nil)
	return err
}
