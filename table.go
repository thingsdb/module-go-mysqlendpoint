package main

import (
	"fmt"
	"io"

	"github.com/dolthub/go-mysql-server/sql"
	"github.com/thingsdb/go-thingsdb"
)

type LiveTable struct {
	dbName     string
	name       string
	schema     sql.Schema
	scope      string
	queryAll   string
	queryCount string
	conn       *thingsdb.Conn
	fks        []sql.ForeignKeyConstraint
}

var _ sql.Table = (*LiveTable)(nil)
var _ sql.ForeignKeyTable = (*LiveTable)(nil)
var _ sql.StatisticsTable = (*LiveTable)(nil)

func (t *LiveTable) Name() string {
	return t.name
}

func (t *LiveTable) String() string {
	return t.name
}

func (t *LiveTable) Schema() sql.Schema {
	return t.schema
}

func (t *LiveTable) Collation() sql.CollationID {
	return sql.Collation_Default
}

// Partitions returns a single partition since ThingsDB executes the query as a single stream
func (t *LiveTable) Partitions(ctx *sql.Context) (sql.PartitionIter, error) {
	return &singlePartitionIter{done: false}, nil
}

// PartitionRows executes the live query against ThingsDB and returns the streaming RowIter
func (t *LiveTable) PartitionRows(ctx *sql.Context, part sql.Partition) (sql.RowIter, error) {
	scope, vars, err := t.resolveQueryContext(ctx)
	if err != nil {
		return nil, err
	}
	var rawRows [][]any
	if err := t.conn.QueryInto(scope, t.queryAll, vars, &rawRows); err != nil {
		return nil, fmt.Errorf("live query error on table '%s': %w", t.Name(), err)
	}
	return &thingsDBRowIter{
		rows: rawRows,
		idx:  0,
	}, nil
}

// GetDeclaredForeignKeys implements sql.ForeignKeyTable
func (t *LiveTable) GetDeclaredForeignKeys(ctx *sql.Context) ([]sql.ForeignKeyConstraint, error) {
	return t.fks, nil
}

// AddForeignKey implements sql.ForeignKeyTable
func (t *LiveTable) AddForeignKey(ctx *sql.Context, fk sql.ForeignKeyConstraint) error {
	return fmt.Errorf("table '%s' is read-only: adding foreign keys is not supported", t.name)
}

// UpdateForeignKey implements sql.ForeignKeyTable
func (t *LiveTable) UpdateForeignKey(ctx *sql.Context, fkName string, fk sql.ForeignKeyConstraint) error {
	return fmt.Errorf("table '%s' is read-only: updating foreign keys is not supported", t.name)
}

// DropForeignKey implements sql.ForeignKeyTable
func (t *LiveTable) DropForeignKey(ctx *sql.Context, fkName string) error {
	return fmt.Errorf("table '%s' is read-only: dropping foreign keys is not supported", t.name)
}

// CreateIndexForForeignKey implements sql.ForeignKeyTable
func (t *LiveTable) CreateIndexForForeignKey(ctx *sql.Context, idxDef sql.IndexDef) error {
	return nil
}

// GetForeignKeyEditor implements sql.ForeignKeyTable
func (t *LiveTable) GetForeignKeyEditor(ctx *sql.Context) sql.ForeignKeyEditor {
	return nil
}

// GetIndexes implements sql.IndexAddressableTable (embedded by sql.ForeignKeyTable)
func (t *LiveTable) GetIndexes(ctx *sql.Context) ([]sql.Index, error) {
	return nil, nil
}

// GetReferencedForeignKeys implements sql.ForeignKeyTable
func (t *LiveTable) GetReferencedForeignKeys(ctx *sql.Context) ([]sql.ForeignKeyConstraint, error) {
	return nil, nil
}

// IndexedAccess implements sql.IndexAddressableTable (embedded by sql.ForeignKeyTable)
func (t *LiveTable) IndexedAccess(ctx *sql.Context, lookup sql.IndexLookup) sql.IndexedTable {
	return nil
}

// PreciseMatch implements sql.IndexAddressableTable
func (t *LiveTable) PreciseMatch() bool {
	return true
}

// DataLength returns the estimated size in bytes (can safely return 0).
func (t *LiveTable) DataLength(ctx *sql.Context) (uint64, error) {
	return 0, nil
}

// RowCount returns (count, exact, err).
func (t *LiveTable) RowCount(ctx *sql.Context) (uint64, bool, error) {
	if t.queryCount == "" {
		return 0, false, nil
	}
	scope, vars, err := t.resolveQueryContext(ctx)
	if err != nil {
		return 0, false, err
	}
	var count uint64
	if err := t.conn.QueryInto(scope, t.queryCount, vars, &count); err != nil {
		return 0, false, fmt.Errorf("live count query error on table '%s': %w", t.Name(), err)
	}
	return count, true, nil
}

func NewLiveTable(dbName string, tblDef Table, conn *thingsdb.Conn) (*LiveTable, error) {
	sqlCols := make(sql.Schema, 0, len(tblDef.Columns))
	var fks []sql.ForeignKeyConstraint

	for _, col := range tblDef.Columns {
		sqlType, err := mapColumnType(col.Type)
		if err != nil {
			return nil, fmt.Errorf("table '%s' column '%s': %w", tblDef.Name, col.Name, err)
		}

		sqlCols = append(sqlCols, &sql.Column{
			Name:          col.Name,
			Type:          sqlType,
			Source:        tblDef.Name,
			PrimaryKey:    col.Primary,
			Nullable:      col.Nullable,
			Comment:       col.Comment,
			AutoIncrement: false,
		})

		if col.ForeignKey != nil && col.ForeignKey.Table != "" && col.ForeignKey.Column != "" {
			fks = append(fks, sql.ForeignKeyConstraint{
				Name:           fmt.Sprintf("fk_%s_%s", tblDef.Name, col.Name),
				Database:       dbName,
				Table:          tblDef.Name,
				Columns:        []string{col.Name},
				ParentDatabase: dbName,
				ParentTable:    col.ForeignKey.Table,
				ParentColumns:  []string{col.ForeignKey.Column},
				OnUpdate:       sql.ForeignKeyReferentialAction_NoAction,
				OnDelete:       sql.ForeignKeyReferentialAction_NoAction,
			})
		}
	}

	return &LiveTable{
		dbName:     dbName,
		name:       tblDef.Name,
		schema:     sqlCols,
		scope:      tblDef.Scope,
		queryAll:   tblDef.Select.All,
		queryCount: tblDef.Select.Count,
		conn:       conn,
		fks:        fks,
	}, nil
}

// ---------------- Partition Iterator ----------------
type singlePartitionIter struct {
	done bool
}

type singlePartition struct{}

func (singlePartition) Key() []byte {
	return []byte("thingsdb_partition_0")
}

func (i *singlePartitionIter) Next(*sql.Context) (sql.Partition, error) {
	if i.done {
		return nil, io.EOF
	}
	i.done = true
	return singlePartition{}, nil
}

func (i *singlePartitionIter) Close(*sql.Context) error {
	return nil
}

// ---------------- Row Iterator ----------------
type thingsDBRowIter struct {
	rows [][]any
	idx  int
}

func (i *thingsDBRowIter) Next(ctx *sql.Context) (sql.Row, error) {
	if i.idx >= len(i.rows) {
		return nil, io.EOF
	}

	raw := i.rows[i.idx]
	i.idx++

	row := make(sql.Row, len(raw))
	copy(row, raw)

	return row, nil
}

func (i *thingsDBRowIter) Close(*sql.Context) error {
	i.rows = nil
	return nil
}

// ---------------- Context user & scope ----------------
func (t *LiveTable) resolveQueryContext(ctx *sql.Context) (string, map[string]any, error) {
	user := ctx.Client().User

	scope := ""
	if userScope, ok := userScopes.Get(user); ok && userScope != "" {
		scope = userScope
	}
	if t.scope != "" {
		scope = t.scope
	}
	if scope == "" {
		return "", nil, fmt.Errorf("missing ThingsDB scope for user %q on table %q: configure a scope on the user or set a fixed scope on the table", user, t.Name())
	}

	vars := map[string]any{
		"username": user,
	}
	return scope, vars, nil
}
