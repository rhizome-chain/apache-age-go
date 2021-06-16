package main

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

type Age struct {
	db        *sql.DB
	graphName string
}

type AgeTx struct {
	age *Age
	tx  *sql.Tx
}

/**
@param dsn host=127.0.0.1 port=5432 dbname=postgres user=postgres password=agens sslmode=disable
*/
func ConnectAge(graphName string, dsn string) (*Age, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	age := &Age{db: db, graphName: graphName}
	// _, err = age.GetReady()
	// if err != nil {
	// 	age.Close()
	// 	return nil, err
	// }
	_, err = age.GetReady()

	if err != nil {
		db.Close()
		age = nil
	}

	return age, err
}

func NewAge(graphName string, db *sql.DB) *Age {
	return &Age{db: db, graphName: graphName}
}

func (age *Age) GetReady() (bool, error) {
	tx, err := age.db.Begin()
	if err != nil {
		return false, err
	}

	_, err = tx.Exec("LOAD 'age';")
	if err != nil {
		return false, err
	}

	_, err = tx.Exec("SET search_path = ag_catalog, '$user', public;")
	if err != nil {
		return false, err
	}

	var count int = 0

	err = tx.QueryRow("SELECT count(*) FROM ag_graph WHERE name=$1", age.graphName).Scan(&count)

	if err != nil {
		return false, err
	}

	if count == 0 {
		_, err = tx.Exec("SELECT create_graph($1);", age.graphName)
		if err != nil {
			return false, err
		}
	}

	tx.Commit()

	return true, nil
}

func (a *Age) Close() error {
	return a.db.Close()
}

func (a *Age) DB() *sql.DB {
	return a.db
}

func (a *Age) Begin() (*AgeTx, error) {
	ageTx := &AgeTx{age: a}
	tx, err := a.db.Begin()
	if err != nil {
		return nil, err
	}
	ageTx.tx = tx
	return ageTx, err
}

func (t *AgeTx) Commit() error {
	return t.tx.Commit()
}
func (t *AgeTx) Rollback() error {
	return t.tx.Rollback()
}

func (a *AgeTx) Exec(stmt string, args ...interface{}) (sql.Result, error) {
	return a.tx.Exec(stmt, args...)
}

/** CREATE , DROP .... */
func (a *AgeTx) ExecCypher(cypher string, args ...interface{}) error {
	cypherStmt := fmt.Sprintf(cypher, args...)
	stmt := fmt.Sprintf("SELECT * from cypher('%s', $$ %s $$) as (v agtype);",
		a.age.graphName, cypherStmt)

	_, err := a.tx.Exec(stmt)
	if err != nil {
		return err
	}
	return nil
}

/** MATCH .... RETURN .... */
func (a *AgeTx) QueryCypher(cypher string, args ...interface{}) (*CypherCursor, error) {
	cypherStmt := fmt.Sprintf(cypher, args...)
	stmt := fmt.Sprintf("SELECT * from cypher('%s', $$ %s $$) as (v agtype);",
		a.age.graphName, cypherStmt)

	rows, err := a.tx.Query(stmt)
	if err != nil {
		return nil, err
	} else {
		return NewCypherCursor(rows), nil
	}
}

type CypherCursor struct {
	rows        *sql.Rows
	unmarshaler *AGUnmarshaler
}

func NewCypherCursor(rows *sql.Rows) *CypherCursor {
	return &CypherCursor{rows: rows, unmarshaler: NewAGUnmarshaler()}
}

func (c *CypherCursor) All() ([]Entity, error) {
	defer c.rows.Close()

	ens := []Entity{}

	for c.rows.Next() {
		entity, err := c.GetRow()
		if err != nil {
			return ens, err
		}
		ens = append(ens, entity)
	}

	return ens, nil
}

func (c *CypherCursor) Next() bool {
	return c.rows.Next()
}

func (c *CypherCursor) GetRow() (Entity, error) {
	var gstr string
	err := c.rows.Scan(&gstr)
	if err != nil {
		return nil, fmt.Errorf("CypherCursor.GetRow:: %s", err)
	}
	return c.unmarshaler.unmarshal(gstr)
}

func (c *CypherCursor) Close() error {
	return c.rows.Close()
}

// func (a *Age) ParsAgeResult(resultStr string) Entity {
// 	return NewAGUnmarshaler().unmarshal(resultStr)
// }
