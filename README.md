# MySQL Endpoint Module (Go)

MySQL Endpoint module written using the [Go language](https://golang.org).

With this module, ThingsDB can expose data as-if it is MySQL.

## Installation

Install the module by running the following command in the `@thingsdb` scope:

```javascript
new_module('mysqlendpoint', 'github.com/thingsdb/module-go-mysqlendpoint');
```

Optionally, you can choose a specific version by adding a `@` followed with the release tag. For example: `@v0.1.0`.

## Configuration

Property            | Type            | Description
------------------- | --------------- | -----------
`thingsdb_token`    | str (required)  | Token with `QUERY` + `CHANGE` privileges in at least the `@thinhsdb` scope.
`thingsdb_port`     | int (optional)  | Client port of the thingsdb node. Defaults to `9200`.
`thingsdb_timeout`  | int (optional)  | Timeout in seconds for thingsdb queries. Defaults to `10` seconds.
`schema_scope`      | str (optional)  | Collection scope for storing the schema. Defaults to `//schema`.
`mysql_bind_address`| str (optional)  | Bind address for the MySQL server endpoint. Defaults to `0.0.0.0`.
`mysql_listen_port` | str (optional)  | Listen port for the MySQL server endpoint. Defaults to `3306`.


Example configuration:

```javascript
// Restrict usage to the @thingsdb scope; This is not required, but recommended
set_module_scope('mysqlendpoint', '/t');

set_module_conf('mysqlendpoint', {
    thingsdb_token: '98c3fzHAIwYILw7JLyJbvH',
});
```

## Exposed functions

Name              | Description
----------------- | -----------
[info](#info)     | Returns info for the Module configuration database schema.
[reload](#reload) | Reload the schema. Can be used after schema changes.

### info

Syntax: `info()`

#### Example:

```javascript
mysqlendpoint.info();
```

### reload

Syntax: `reload()`

#### Example:

```javascript
mysqlendpoint.reload();
```

#### Return value:

`nil`
