package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/sourcegraph/conc/pool"
	"github.com/srabraham/strava-stats/types"
)

var (
	dbUser     = flag.String("db-user", "", "User for database connection")
	dbPassword = flag.String("db-password", "", "Password for database connection")
	dbHost     = flag.String("db-host", "127.0.0.1", "Host for database connection")
	dbPort     = flag.String("db-port", "3306", "Port for database connection")
	dbName     = flag.String("db-name", "strava", "Name for database")

	inputJson = flag.String("input-json", "", "Input file with Strava data")
)

const (
	dropAthletesTable = `
drop table if exists Athletes
`
	createAthletesTable = `
create table Athletes (
    ID bigint
    ,FirstName varchar(255)
    ,LastName varchar(255)
    ,City varchar(255)
)
`
	dropActivtiesTable = `
drop table if exists Activities
`
	createActivitiesTable = `
create table Activities (
    ID bigint
    ,AthleteID bigint
    ,Name varchar(255)
)
`
)

func main() {
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	cfg := mysql.Config{
		User:   *dbUser,
		Passwd: *dbPassword,
		Net:    "tcp",
		Addr:   fmt.Sprintf("%v:%v", *dbHost, *dbPort),
	}
	db, err := sqlx.Connect("mysql", cfg.FormatDSN())
	if err != nil {
		log.Fatal(err)
	}
	db.MustExecContext(ctx, "create database if not exists "+*dbName)
	cfg.DBName = *dbName
	db, err = sqlx.Connect("mysql", cfg.FormatDSN())
	if err != nil {
		log.Fatal(err)
	}
	db.MustExecContext(ctx, dropAthletesTable)
	db.MustExecContext(ctx, createAthletesTable)
	db.MustExecContext(ctx, dropActivtiesTable)
	db.MustExecContext(ctx, createActivitiesTable)

	b, err := os.ReadFile(*inputJson)
	if err != nil {
		log.Fatal(err)
	}
	var sd types.StravaData
	if err = json.Unmarshal(b, &sd); err != nil {
		log.Fatal(err)
	}
	_, err = db.NamedExecContext(ctx,
		"insert into Athletes (ID, FirstName, LastName, City) values (:id, :firstname, :lastname, :city)",
		sd.Athlete)
	if err != nil {
		log.Fatal(err)
	}

	// speedy concurrent inserts, up to n at once
	p := pool.New().WithMaxGoroutines(50).WithErrors()
	for _, act := range sd.Activities {
		act := act
		p.Go(func() error {
			_, err = db.NamedExecContext(ctx,
				"insert into Activities (ID, AthleteID, Name) values (:id, :athlete.id, :name)",
				act)
			if err != nil {
				return fmt.Errorf("for activity %v: %w", act, err)
			}
			return nil
		})
	}
	err = p.Wait()
	if err != nil {
		log.Fatal(err)
	}
}
