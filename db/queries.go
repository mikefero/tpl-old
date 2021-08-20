package db

// Tables
const leaguesTable = `CREATE TABLE leagues (
  id     INTEGER PRIMARY KEY AUTOINCREMENT
                 NOT NULL,
  name   STRING  NOT NULL,
  active BOOLEAN NOT NULL);`

const featuresTables = `CREATE TABLE features (
  id       INTEGER PRIMARY KEY AUTOINCREMENT,
  features STRING  NOT NULL
                   UNIQUE ON CONFLICT ABORT);`

const machineManufacturersTable = `CREATE TABLE machine_manufacturers (
  id               INTEGER PRIMARY KEY ON CONFLICT IGNORE
                           UNIQUE,
  name             STRING  NOT NULL,
  full_name        STRING  NOT NULL,
  updated_at INTEGER NOT NULL);`

const machinesTable = `CREATE TABLE machines (
  opdb_id              STRING  PRIMARY KEY ON CONFLICT IGNORE
                               NOT NULL,
  manufacturer_id      INTEGER REFERENCES machine_manufacturer (id)
                               NOT NULL,
  ipdb_id              INTEGER,
  features_id          INTEGER REFERENCES features (id),
  name                 STRING  NOT NULL,
  manufacture_date     INTEGER,
  backglass_image_uuid TEXT,
  updated_at           INTEGER NOT NULL,
  active               BOOLEAN NOT NULL);`

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

// Features table queries
const sqlSelectIdFromFeatures = `SELECT id
  FROM features
  WHERE features = ?`

const sqlSelectFeatures = `SELECT features
  FROM features
  WHERE id = ?`

const sqlInsertFeatures = `INSERT INTO features (features)
    VALUES (?);`

// Manufacturer table queries
const sqlInsertMachineManufacturers = `INSERT INTO machine_manufacturers (
  id, name, full_name, updated_at)
  VALUES (?, ?, ?, ?);`

// Machine queries
const sqlInsertMachines = `INSERT INTO machines (
  opdb_id, manufacturer_id, ipdb_id, features_id, name, manufacture_date, backglass_image_uuid, updated_at, active)
  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);`

const sqlResetActiveMachines = `UPDATE machines SET active = false`

const sqlUpdateActiveMachines = `UPDATE machines
  SET active = true
  WHERE opdb_id = ?`

const sqlSelectAllActiveMachines = `SELECT opdb_id, manufacturer_id, ipdb_id, features_id, name, manufacture_date, backglass_image_uuid, updated_at
  FROM machines
  WHERE active = true`
