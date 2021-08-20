package db

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/mikefero/tpl/log"
	json "github.com/tidwall/gjson"
)

const tplDatabasePath = "db/tpl.db"
const tplInitialOpdbExportPath = "db/opdb.json"
const tplInitialLocationId = 4907 // The Pinball Lounge

type Machine struct {
	OpdbId             string
	ManufacturerId     int
	IpdbId             sql.NullInt64
	FeaturesId         sql.NullInt64
	Name               string
	ManufactureDate    sql.NullInt64
	BackglassImageUuid sql.NullString
	UpdatedAt          int
	Active             bool
}

var stmtSelectIdFromFeatures *sql.Stmt
var stmtSelectAllActiveMachines *sql.Stmt
var stmtSelectFeatures *sql.Stmt

func GetAllActiveMachines() []Machine {
	rows, err := stmtSelectAllActiveMachines.Query()
	if err != nil {
		log.WithFields(log.Fields{
			"statement": sqlSelectAllActiveMachines,
			"error":     err,
		}).Error("unable to execute prepared SQL statement")
		return nil
	}
	defer rows.Close()

	var activeMachines []Machine
	for rows.Next() {
		var activeMachine Machine
		activeMachine.Active = true
		if err := rows.Scan(&activeMachine.OpdbId,
			&activeMachine.ManufacturerId,
			&activeMachine.IpdbId,
			&activeMachine.FeaturesId,
			&activeMachine.Name,
			&activeMachine.ManufactureDate,
			&activeMachine.BackglassImageUuid,
			&activeMachine.UpdatedAt); err != nil {
			log.WithFields(log.Fields{
				"statement": sqlSelectAllActiveMachines,
				"result":    rows,
				"error":     err,
			}).Warn("unable to scan result for active machine")
		} else {
			activeMachines = append(activeMachines, activeMachine)
		}
	}
	if err := rows.Err(); err != nil {
		log.WithFields(log.Fields{
			"statement": sqlSelectAllActiveMachines,
			"result":    rows,
			"error":     err,
		}).Error("unable to scan result for active machine")
	}

	return activeMachines
}

func GetFeatures(id int) string {
	var features string
	err := stmtSelectFeatures.QueryRow(id).Scan(&features)
	if err != nil {
		log.WithFields(log.Fields{
			"statement": sqlSelectFeatures,
			"error":     err,
		}).Error("unable to execute prepared SQL statement")
	}

	return features
}

func closePreparedMachinesStatements() {
	log.Debug("closing prepared machines statements")
	stmtSelectIdFromFeatures.Close()
	stmtSelectFeatures.Close()
	stmtSelectAllActiveMachines.Close()
	log.Debug("prepared machines statements closed")
}

func prepareMachinesStatements() {
	log.Debug("preparing machines statements")
	stmtSelectIdFromFeatures = prepare(sqlSelectIdFromFeatures)
	stmtSelectFeatures = prepare(sqlSelectFeatures)
	stmtSelectAllActiveMachines = prepare(sqlSelectAllActiveMachines)
	log.Debug("machines statements prepared")
}

func initPinballMachineFeaturesTable(path string) {
	log.WithFields(log.Fields{
		"path": path,
	}).Debug("initializing features tables")
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

	tx, err := session.Begin()
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Panic("unable to begin transaction TPL features table creation and initialization")
	}

	txExec(tx, featuresTables)
	stmtInsertFeatures := txPrepare(tx, sqlInsertFeatures)
	defer stmtInsertFeatures.Close()

	json.ParseBytes(data).ForEach(func(key, value json.Result) bool {
		featuresArray := value.Get("features").Array()
		if len(featuresArray) > 0 {
			var features []string
			for _, feature := range featuresArray {
				features = append(features, feature.String())
			}
			_, err := stmtInsertFeatures.Exec(
				strings.Join(features, ","))
			log.WithFields(log.Fields{
				"statement": sqlInsertFeatures,
				"features":  features,
			}).Trace("transactionally execute features SQL insert statement")
			if err != nil {
				log.WithFields(log.Fields{
					"statement": sqlInsertFeatures,
					"feature":   features,
					"error":     err,
				}).Trace("unable to transactionally execute features SQL insert statement")
			}
		}
		return true
	})

	err = tx.Commit()
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Panic("unable to commit transaction for TPL features table creation and initialization")
	}
}

func initPinballMachineTables(tx *sql.Tx, path string) {
	log.WithFields(log.Fields{
		"path": path,
	}).Debug("initializing machine and machine manufacturers tables")
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

	txExec(tx, machineManufacturersTable)
	txExec(tx, machinesTable)

	stmtInsertMachines := txPrepare(tx, sqlInsertMachines)
	stmtInsertMachineManufacturers := txPrepare(tx, sqlInsertMachineManufacturers)
	stmtSelectIdFromFeatures := prepare(sqlSelectIdFromFeatures)
	defer stmtInsertMachines.Close()
	defer stmtInsertMachineManufacturers.Close()
	defer stmtSelectIdFromFeatures.Close()
	re := regexp.MustCompile(`[0-9a-fA-F]{8}\-[0-9a-fA-F]{4}\-[0-9a-fA-F]{4}\-[0-9a-fA-F]{4}\-[0-9a-fA-F]{12}`)

	json.ParseBytes(data).ForEach(func(key, value json.Result) bool {
		opdbId := value.Get("opdb_id")
		ipdbId := value.Get("ipdb_id")
		name := value.Get("name")
		mfrDate := value.Get("manufacture_date")
		backglassImageUrl := value.Get("images.#(type=backglass).urls.large")
		updatedAtValue, _ := time.Parse("2006-01-02", value.Get("updated_at").String())
		mfrId := value.Get("manufacturer.manufacturer_id")
		mfrName := value.Get("manufacturer.name")
		mfrFullName := value.Get("manufacturer.full_name")
		mfrUpdatedAtValue, _ := time.Parse("2006-01-02", value.Get("manufacturer.updated_at").String())

		// Determine the features id
		featuresArray := value.Get("features").Array()
		var featuresId sql.NullInt64
		if len(featuresArray) > 0 {
			var features []string
			for _, feature := range featuresArray {
				features = append(features, feature.String())
			}

			featuresId.Valid = true
			err := stmtSelectIdFromFeatures.QueryRow(strings.Join(features, ",")).Scan(&featuresId.Int64)
			if err != nil {
				log.WithFields(log.Fields{
					"statement": sqlSelectIdFromFeatures,
					"error":     err,
				}).Error("unable to execute prepared SQL statement")
				featuresId.Valid = false
			}
		}

		_, err := stmtInsertMachines.Exec(opdbId.String(),
			mfrId.Int(),
			getValueInt(ipdbId),
			featuresId,
			name.String(),
			getValueTime(mfrDate),
			getValueStringRegex(backglassImageUrl, re),
			updatedAtValue.Unix(),
			false)
		log.WithFields(log.Fields{
			"statement":         sqlInsertMachines,
			"opdb_id":           opdbId.String(),
			"manufacturer_id":   mfrId.Int(),
			"ipdb_id":           ipdbId.String(),
			"name":              name.String(),
			"manufacture_date":  getValueTime(mfrDate),
			"backglassImageUrl": backglassImageUrl.String(),
			"updated_at":        updatedAtValue.Unix(),
			"active":            false,
		}).Trace("transactionally execute machines SQL insert statement")
		if err != nil {
			log.WithFields(log.Fields{
				"statement":            sqlInsertMachines,
				"opdb_id":              opdbId.String(),
				"manufacturer_id":      mfrId.Int(),
				"ipdb_id":              getValueInt(ipdbId),
				"name":                 name.String(),
				"manufacture_date":     mfrDate.String(),
				"backglass_image_uuid": getValueStringRegex(backglassImageUrl, re),
				"updated_at":           updatedAtValue.Unix(),
				"active":               false,
				"error":                err,
			}).Panic("unable to transactionally execute machines SQL insert statement")
		}

		_, err = stmtInsertMachineManufacturers.Exec(
			mfrId.Int(),
			mfrName.String(),
			mfrFullName.String(),
			mfrUpdatedAtValue.Unix())
		log.WithFields(log.Fields{
			"statement":  sqlInsertMachineManufacturers,
			"id":         mfrId.Int(),
			"name":       mfrName.String(),
			"full_name":  mfrFullName.String(),
			"updated_at": mfrUpdatedAtValue.Unix(),
		}).Trace("transactionally execute machine_manufacturers SQL insert statement")
		if err != nil {
			log.WithFields(log.Fields{
				"statement":  sqlInsertMachineManufacturers,
				"id":         mfrId.Int(),
				"name":       mfrName.String(),
				"full_name":  mfrFullName.String(),
				"updated_at": mfrUpdatedAtValue.Unix(),
				"error":      err,
			}).Panic("unable to transactionally execute machine_manufacturers SQL insert statement")
		}

		return true
	})

	log.Debug("machine and machine manufacturers tables initialized")
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
		return
	}

	txExec(tx, sqlResetActiveMachines)
	stmtUpdateActiveMachines := txPrepare(tx, sqlUpdateActiveMachines)
	defer stmtUpdateActiveMachines.Close()

	json.GetBytes(data, "machines.#.opdb_id").ForEach(func(key, value json.Result) bool {
		opdbId := value.String()

		_, err := stmtUpdateActiveMachines.Exec(opdbId)
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
