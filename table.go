package main

import (
	"fmt"
	"io"

	"github.com/dolthub/go-mysql-server/memory"
	"github.com/dolthub/go-mysql-server/sql"
	"github.com/thingsdb/go-thingsdb"
)

type LiveTable struct {
	*memory.Table // provides UnderlyingTable() *memory.Table and satisfies memory.MemTable
	scope         string
	query         string
	conn          *thingsdb.Conn
}

func NewLiveTable(db *memory.Database, tblDef Table, conn *thingsdb.Conn) (*LiveTable, error) {
	sqlCols := make(sql.Schema, 0, len(tblDef.Columns))

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
	}

	pkSchema := sql.NewPrimaryKeySchema(sqlCols)

	// memory.NewTable creates an empty table structure without any rows
	memTable := memory.NewTable(db, tblDef.Name, pkSchema, nil)

	return &LiveTable{
		Table: memTable,
		scope: tblDef.Scope,
		query: tblDef.Select.All,
		conn:  conn,
	}, nil
}

// Override Partitions to return our single partition token
func (t *LiveTable) Partitions(ctx *sql.Context) (sql.PartitionIter, error) {
	return &singlePartitionIter{done: false}, nil
}

// Override PartitionRows so it streams live from ThingsDB instead of memory.Table's internal store
func (t *LiveTable) PartitionRows(ctx *sql.Context, p sql.Partition) (sql.RowIter, error) {
	var rawRows [][]any

	if err := t.conn.QueryInto(t.scope, t.query, nil, &rawRows); err != nil {
		return nil, fmt.Errorf("live query error on table '%s': %w", t.Name(), err)
	}

	return &thingsDBRowIter{
		rows: rawRows,
		idx:  0,
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
