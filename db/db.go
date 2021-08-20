package db

import (
	"database/sql"
	"regexp"
	"strings"
	"time"

	"github.com/mikefero/tpl/log"
	"github.com/mikefero/tpl/utils"

	_ "github.com/mattn/go-sqlite3"
	json "github.com/tidwall/gjson"
)

var session *sql.DB

func Close() {
	closeAllPreparedStatements()

	log.Debug("closing TPL database")
	session.Close()
	log.Debug("TPL database closed")
}

func txExec(tx *sql.Tx, sql string) sql.Result {
	result, err := tx.Exec(sql)
	if err != nil {
		log.WithFields(log.Fields{
			"statement": sql,
			"error":     err,
		}).Panic("unable to transactionally execute SQL statement")
		return nil
	}

	return result
}

func prepare(sql string) *sql.Stmt {
	stmt, err := session.Prepare(sql)
	if err != nil {
		log.WithFields(log.Fields{
			"statement": sql,
			"error":     err,
		}).Panic("unable to prepare SQL statement")
	}
	return stmt
}

func txPrepare(tx *sql.Tx, sql string) *sql.Stmt {
	stmt, err := tx.Prepare(sql)
	if err != nil {
		log.WithFields(log.Fields{
			"statement": sql,
			"error":     err,
		}).Panic("unable to transactionally prepare SQL statement")
	}
	return stmt
}

func prepareAllStatements() {
	log.Debug("preparing statements")
	prepareMachinesStatements()
	log.Debug("statements prepared")
}

func closeAllPreparedStatements() {
	log.Debug("closing prepared statements")
	closePreparedMachinesStatements()
	log.Debug("prepared statements closed")
}

func openDatabase(path string) {
	var err error
	log.WithFields(log.Fields{
		"path": path,
	}).Debug("opening TPL database")
	session, err = sql.Open("sqlite3", path)
	if err != nil {
		log.WithFields(log.Fields{
			"path":  path,
			"error": err,
		}).Panic("unable open TPL database")
	}
}

func getValueInt(value json.Result) sql.NullInt64 {
	if value.Value() == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{
		Int64: value.Int(),
		Valid: true,
	}
}

func getValueStringRegex(value json.Result, re *regexp.Regexp) sql.NullString {
	if value.Value() == nil {
		return sql.NullString{}
	}
	return sql.NullString{
		String: string(re.Find([]byte(value.String()))),
		Valid:  true,
	}
}

func getValueTime(value json.Result) sql.NullInt64 {
	if value.Value() == nil || len(strings.TrimSpace(value.String())) == 0 {
		return sql.NullInt64{}
	}
	timestamp, _ := time.Parse("2006-01-02", value.String())
	return sql.NullInt64{
		Int64: timestamp.Unix(),
		Valid: true,
	}
}

func maybeCreateDatabase(path string) {
	if utils.FileExists(path) {
		openDatabase(path)
		return
	}

	log.WithFields(log.Fields{
		"path": path,
	}).Debug("creating TPL database")
	openDatabase(path)

	initPinballMachineFeaturesTable(tplInitialOpdbExportPath)

	// Disable foreign key constraints and begin transaction
	_, err := session.Exec("PRAGMA foreign_keys = off;")
	if err != nil {
		log.WithFields(log.Fields{
			"statement": "PRAGMA foreign_keys = off;",
		}).Panic(err)
	}
	tx, err := session.Begin()
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Panic("unable to begin transaction TPL table creation and machines tables initialization")
	}

	// Create the initial tables for the database
	txExec(tx, leaguesTable)
	txExec(tx, matchesTable)
	txExec(tx, resultsTable)
	txExec(tx, seasonsTable)
	txExec(tx, teamsTable)
	txExec(tx, usersTable)

	// Initialize the machines tables with data from Open Pinball (opdb.org)
	initPinballMachineTables(tx, tplInitialOpdbExportPath)

	// Commit the transaction and re-enable foreign key constraints
	err = tx.Commit()
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Panic("unable to commit transaction for TPL table creation and machines tables initialization")
	}
	_, err = session.Exec("PRAGMA foreign_keys = on;")
	if err != nil {
		log.WithFields(log.Fields{
			"statement": "PRAGMA foreign_keys = on;",
		}).Panic(err)
	}

	// Determine the active machines at the given location
	assignActiveMachines(tplInitialLocationId)

	log.Debug("TPL database created")
}

func init() {
	log.Debug("initializing TPL database")
	maybeCreateDatabase(tplDatabasePath)
	prepareAllStatements()
	log.Debug("TPL database initialized")
}
