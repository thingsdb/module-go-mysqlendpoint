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

## Schema collection
Once configured, this module automatically creates a **`schema`** collection in ThingsDB to define your virtual MySQL database, tables, queries, and users.
<img width="1306" height="621" alt="image" src="https://github.com/user-attachments/assets/d167567f-9778-4a94-bc9f-833da92d1a52" />
### Scope Resolution (Multi-Tenant & Shared Data)

A ThingsDB `scope` can be defined on a **User**, on a **Table**, or both:

* **Table scope takes precedence:** If a `scope` is set on a table, it overrides any user-level scope.
* **User scope as default:** If a table's `scope` is left empty, queries automatically route to the `scope` configured for the authenticated MySQL user.

> **Tip:** This precedence allows you to combine **per-user collections** (tables with no fixed scope) alongside **shared global collections** (tables pinned to a specific scope) within the same MySQL database.

---

### Select Queries (`all` & `count`)

Currently, the `Select` configuration supports two query definitions:

| Field | Expected Return Value | Description |
| :--- | :--- | :--- |
| **`all`** | `list` or `tuple` of rows | Each row must be a list or tuple matching the order and types of the defined table columns. |
| **`count`** | `int` | Must return the total number of rows available for the table. |

#### Dynamic Filtering with `username`

Both `all` and `count` queries are executed with an injected **`username`** variable (`str`) containing the logged-in MySQL client's username. You can reference `username` directly inside your ThingsDB queries to filter rows or compute user-specific values, even when querying a shared collection.

