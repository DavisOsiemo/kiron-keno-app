package main

import (
	"database/sql"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	cron "github.com/robfig/cron/v3"
)

func main() {
	// Fetch XML data
	resp, err := http.Get(endpoint)
	if err != nil {
		log.Fatalf("Failed to fetch data: %v", err)
	}
	defer resp.Body.Close()

	xmlData, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read response: %v", err)
	}

	var events UpcomingEvents
	if err := xml.Unmarshal(xmlData, &events); err != nil {
		log.Fatalf("Failed to parse XML: %v", err)
	}

	// Connect to MySQL
	// mysql -uapps_user -h10.79.224.2 -p'Tb#<M#BnvBc%ur5q'
	dsn := "apps_user:Tb#<M#BnvBc%ur5q@tcp(10.79.224.2:3306)/kiron"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	insertStmt := `
	INSERT INTO keno_events (
		id, event_type, event_number, event_time, finish_time, event_status,
		local_time, utc_time, round_trip_time
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON DUPLICATE KEY UPDATE
		event_type=VALUES(event_type),
		event_number=VALUES(event_number),
		event_time=VALUES(event_time),
		finish_time=VALUES(finish_time),
		event_status=VALUES(event_status),
		local_time=VALUES(local_time),
		utc_time=VALUES(utc_time),
		round_trip_time=VALUES(round_trip_time)
	`

	for _, e := range events.KenoEvents {
		_, err := db.Exec(insertStmt,
			e.ID, e.EventType, e.EventNumber, e.EventTime, e.FinishTime, e.EventStatus,
			events.LocalTime, events.UtcTime, events.RoundTripTime,
		)
		if err != nil {
			log.Printf("Insert failed for ID %d: %v", e.ID, err)
		}
	}

	fmt.Println("✅ Data inserted successfully.")
}

func RunCron() {

	loc, _ := time.LoadLocation("Africa/Nairobi")
	c := cron.New(cron.WithLocation(loc))

	//c.AddFunc("0 4 * * *", ArchiveOddsLive)

	c.Start()
}

// func RunCron() {
// 	c := cron.New()

// 	c.AddFunc("@every 05h05m00s", ArchiveOddsLive)
// 	c.AddFunc("@every 08h00m05s", ArchiveFixtureStatus)
// 	c.AddFunc("@every 24h00m05s", ArchiveFixtures)

// 	c.AddFunc("@every 24h00m00s", FetchFootballMatchFixtures)
// 	c.AddFunc("@every 24h00m00s", FetchBasketballMatchFixtures)
// 	c.AddFunc("@every 24h00m00s", FetchTableTennisMatchFixtures)
// 	c.AddFunc("@every 24h00m00s", FetchTennisMatchFixtures)
// 	c.AddFunc("@every 24h00m00s", FetchRugbyLeagueMatchFixtures)
// 	c.AddFunc("@every 24h00m00s", FetchRugbyUnionMatchFixtures)
// 	c.AddFunc("@every 24h00m00s", FetchCricketMatchFixtures)

// 	c.Start()
// }
