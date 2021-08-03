package db

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"time"

	"github.com/mikefero/tpl/log"
	"github.com/mikefero/tpl/utils"

	_ "github.com/mattn/go-sqlite3"
	json "github.com/tidwall/gjson"
)

const leaguesTable = `CREATE TABLE leagues (
    id     INTEGER PRIMARY KEY AUTOINCREMENT
                   NOT NULL,
    name   STRING  NOT NULL,
    active BOOLEAN NOT NULL);`

const machineManufacturersTable = `CREATE TABLE machine_manufacturers (
    id         INTEGER PRIMARY KEY ON CONFLICT IGNORE
                       UNIQUE,
    name       STRING  NOT NULL,
    full_name  STRING  NOT NULL,
    updated_at INTEGER NOT NULL);`

const insertMachineManufacturers = `INSERT INTO machine_manufacturers (
    id, name, full_name, updated_at)
    VALUES (?, ?, ?, ?);`

const machinesTable = `CREATE TABLE machines (
    opdb_id              STRING  PRIMARY KEY ON CONFLICT IGNORE
                                 NOT NULL,
    manufacturer_id      INTEGER REFERENCES machine_manufacturer (id)
                                 NOT NULL,
    ipdb_id              INTEGER,
    name                 STRING  NOT NULL,
    backglass_image_uuid TEXT,
    updated_at           INTEGER NOT NULL,
		active               BOOLEAN NOT NULL);`

const insertMachines = `INSERT INTO machines (
    opdb_id, manufacturer_id, ipdb_id, name, backglass_image_uuid, updated_at, active)
    VALUES (?, ?, ?, ?, ?, ?, ?);`

const resetActiveMachines = `UPDATE machines SET active = false`
const updateActiveMachines = `UPDATE machines
		SET active = true
    WHERE opdb_id = ?`

const matchesTable = `CREATE TABLE matches (
    id        INTEGER PRIMARY KEY AUTOINCREMENT
                      NOT NULL,
    league_id INTEGER REFERENCES leagues (id)
                      NOT NULL,
    season_id INTEGER REFERENCES seasons (id)
                      NOT NULL,
    team_1_id INTEGER REFERENCES teams (id)
                      NOT NULL,
    team_2_id INTEGER REFERENCES teams (id));`

const resultsTable = `CREATE TABLE results (
    id                    INTEGER PRIMARY KEY AUTOINCREMENT
                                  NOT NULL,
    match_id              INTEGER REFERENCES matches (id)
                                  NOT NULL,
    opdb_id               STRING  NOT NULL
                                  REFERENCES machines (opdb_id),
    team_1_a_player_id    INTEGER REFERENCES users (id),
    team_1_a_player_score INTEGER,
    team_1_b_player_id    INTEGER REFERENCES users (id),
    team_1_b_player_score INTEGER,
    team_1_score          INTEGER,
    team_2_a_player_id    INTEGER REFERENCES users (id),
    team_2_a_player_score INTEGER,
    team_2_b_player_id    INTEGER REFERENCES users (id),
    team_2_b_player_score INTEGER,
    team_2_score          INTEGER);`

const seasonsTable = `CREATE TABLE seasons (
    id         INTEGER PRIMARY KEY AUTOINCREMENT
                       NOT NULL,
    name       STRING  NOT NULL,
    start_date TIME    NOT NULL,
    end_date   TIME);`

const teamsTable = `CREATE TABLE teams (
    id        INTEGER PRIMARY KEY AUTOINCREMENT
                      NOT NULL,
    league_id INTEGER REFERENCES leagues (id)
                      NOT NULL,
    name      STRING  NOT NULL
                      UNIQUE,
    a_player  INTEGER REFERENCES users (id)
                      NOT NULL,
    b_player  INTEGER REFERENCES users (id)
                      NOT NULL,
    active    BOOLEAN NOT NULL);`

const usersTable = `CREATE TABLE users (
    id        INTEGER PRIMARY KEY AUTOINCREMENT
                      NOT NULL,
    league_id INTEGER REFERENCES leagues (id)
                      NOT NULL,
    email     STRING  UNIQUE
                      NOT NULL,
    password  STRING  NOT NULL,
    name      STRING  NOT NULL,
    initials  STRING,
    active    BOOLEAN NOT NULL);`

const tplDatabasePath = "db/tpl.db"
const tplInitialOpdbExportPath = "db/opdb.json"
const tplInitialLocationId = 4907 // The Pinball Lounge

var session *sql.DB

func Close() {
	session.Close()
	log.Debug("TPL database closed")
}

func txExec(tx *sql.Tx, sql string) {
	_, err := tx.Exec(sql)
	if err != nil {
		log.WithFields(log.Fields{
			"statement": sql,
			"error":     err,
		}).Panic("unable to transactionally execute SQL statement")
	}
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

func initMachineTables(tx *sql.Tx, path string) {
	log.WithFields(log.Fields{
		"path": path,
	}).Debug("initializing machine tables")
	file, err := os.Open("db/opdb.json")
	if err != nil {
		log.WithFields(log.Fields{
			"path":  path,
			"error": err,
		}).Panic("failed reading Open Pinball Database JSON file")
	}
	defer file.Close()

	data, err := ioutil.ReadAll(file)
	if err != nil {
		log.WithFields(log.Fields{
			"path":  path,
			"error": err,
		}).Panic("failed reading Open Pinball Database JSON file")
	}

	machsStmt := txPrepare(tx, insertMachines)
	machMfrStmt := txPrepare(tx, insertMachineManufacturers)
	re := regexp.MustCompile(`[0-9a-fA-F]{8}\-[0-9a-fA-F]{4}\-[0-9a-fA-F]{4}\-[0-9a-fA-F]{4}\-[0-9a-fA-F]{12}`)

	json.ParseBytes(data).ForEach(func(key, value json.Result) bool {
		opdbId := value.Get("opdb_id")
		ipdbId := value.Get("ipdb_id")
		name := value.Get("name")
		backglassImageUrl := value.Get("images.#(type=backglass).urls.large")
		updatedAtValue, _ := time.Parse("2006-01-02", value.Get("updated_at").String())
		mfrId := value.Get("manufacturer.manufacturer_id")
		mfrName := value.Get("manufacturer.name")
		mfrFullName := value.Get("manufacturer.full_name")
		mfrUpdatedAtValue, _ := time.Parse("2006-01-02", value.Get("manufacturer.updated_at").String())

		_, err := machsStmt.Exec(opdbId.String(),
			mfrId.Int(),
			getValueInt(ipdbId),
			name.String(),
			getValueStringRegex(backglassImageUrl, re),
			updatedAtValue.Unix(),
			false)
		log.WithFields(log.Fields{
			"statement":         insertMachines,
			"opdb_id":           opdbId.String(),
			"manufacturer_id":   mfrId.Int(),
			"ipdb_id":           ipdbId.String(),
			"name":              name.String(),
			"backglassImageUrl": backglassImageUrl.String(),
			"updated_at":        updatedAtValue.Unix(),
			"active":            false,
		}).Trace("transactionally execute machines SQL insert statement")
		if err != nil {
			log.WithFields(log.Fields{
				"statement":            insertMachines,
				"opdb_id":              opdbId.String(),
				"manufacturer_id":      mfrId.Int(),
				"ipdb_id":              getValueInt(ipdbId),
				"name":                 name.String(),
				"backglass_image_uuid": getValueStringRegex(backglassImageUrl, re),
				"updated_at":           updatedAtValue.Unix(),
				"active":               false,
				"error":                err,
			}).Panic("unable to transactionally execute machines SQL insert statement")
		}
		_, err = machMfrStmt.Exec(
			mfrId.Int(),
			mfrName.String(),
			mfrFullName.String(),
			mfrUpdatedAtValue.Unix())
		log.WithFields(log.Fields{
			"statement":  insertMachineManufacturers,
			"id":         mfrId.Int(),
			"name":       mfrName.String(),
			"full_name":  mfrFullName.String(),
			"updated_at": mfrUpdatedAtValue.Unix(),
		}).Trace("transactionally execute machine_manufacturers SQL insert statement")
		if err != nil {
			log.WithFields(log.Fields{
				"statement":  insertMachineManufacturers,
				"id":         mfrId.Int(),
				"name":       mfrName.String(),
				"full_name":  mfrFullName.String(),
				"updated_at": mfrUpdatedAtValue.Unix(),
				"error":      err,
			}).Panic("unable to transactionally execute machine_manufacturers SQL insert statement")
		}

		return true
	})
	log.Debug("machine tables initialized")
}

func assignActiveMachines(locationId int) {
	log.WithFields(log.Fields{
		"locationId": locationId,
	}).Debug("assign active machines using Pinball Map API")

	// Use the API from Pinball Maps to determine the active machines
	pinballMapLocationsApiEndpoint := fmt.Sprintf("%s%d%s",
		"https://pinballmap.com/api/v1/locations/",
		locationId,
		"/machine_details.json")
	response, err := http.Get(pinballMapLocationsApiEndpoint)
	if err != nil {
		log.WithFields(log.Fields{
			"locationId": locationId,
			"url":        pinballMapLocationsApiEndpoint,
			"error":      err,
		}).Warn("unable to get response from Pinball Map")
		return
	}

	// Extract the JSON body
	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.WithFields(log.Fields{
			"response": response,
			"error":    err,
		}).Warn("unable to parse the body of the response from Pinball Map")
		return
	}

	// JSON body may contain errors; ensure failure did not occur
	errors := json.GetBytes(data, "errors")
	if errors.Value() != nil {
		log.WithFields(log.Fields{
			"errors": errors.Value(),
		}).Warn("unable to get machine listing from Pinball Map")
		return
	}

	tx, err := session.Begin()
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Warn("unable to begin transaction for assigning active machines")
	}
	txExec(tx, resetActiveMachines)

	machActiveStmt := txPrepare(tx, updateActiveMachines)
	json.GetBytes(data, "machines.#.opdb_id").ForEach(func(key, value json.Result) bool {
		opdbId := value.String()

		_, err := machActiveStmt.Exec(opdbId)
		log.WithFields(log.Fields{
			"opdb_id": opdbId,
		}).Trace("transactionally execute active machines SQL update statement")
		if err != nil {
			log.WithFields(log.Fields{
				"opdb_id": opdbId,
				"error":   err,
			}).Warn("unable to transactionally execute active machines SQL update statement")
		}

		return true
	})

	err = tx.Commit()
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Warn("unable to commit transaction for assigning active machines")
	}

	log.Debug("finished assigning active machines using Pinball Map API")
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
		}).Panic("unable to begin transaction TPL table creation")
	}

	// Create the initial tables for the database
	txExec(tx, leaguesTable)
	txExec(tx, machineManufacturersTable)
	txExec(tx, machinesTable)
	txExec(tx, matchesTable)
	txExec(tx, resultsTable)
	txExec(tx, seasonsTable)
	txExec(tx, teamsTable)
	txExec(tx, usersTable)

	// Initialize the machines tables with data from Open Pinball (opdb.org)
	initMachineTables(tx, tplInitialOpdbExportPath)

	// Commit the transaction and re-enable foreign key constraints
	err = tx.Commit()
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Panic("unable to commit transaction for TPL table creation")
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
	log.Debug("TPL database initialized")
}
