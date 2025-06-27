package main

import (
	"database/sql"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/robfig/cron/v3"
)

const endpoint = "http://vseintegration.kironinteractive.com:8013/vsegameserver/dataservice/UpcomingEvents?hours=4&type=Keno"

// CustomTime handles timestamps without timezone info
type CustomTime struct {
	time.Time
}

const customLayout = "2006-01-02T15:04:05"

func (ct *CustomTime) UnmarshalXMLAttr(attr xml.Attr) error {
	t, err := time.Parse(customLayout, attr.Value)
	if err != nil {
		return err
	}
	ct.Time = t
	return nil
}

type UpcomingEvents struct {
	XMLName       xml.Name    `xml:"UpcomingEvents"`
	LocalTime     CustomTime  `xml:"LocalTime,attr"`
	UtcTime       CustomTime  `xml:"UtcTime,attr"`
	RoundTripTime CustomTime  `xml:"RoundTripTime,attr"`
	KenoEvents    []KenoEvent `xml:"KenoEvent"`
}

type KenoEvent struct {
	ID          int64      `xml:"ID,attr"`
	EventType   string     `xml:"EventType,attr"`
	EventNumber string     `xml:"EventNumber,attr"`
	EventTime   CustomTime `xml:"EventTime,attr"`
	FinishTime  CustomTime `xml:"FinishTime,attr"`
	EventStatus string     `xml:"EventStatus,attr"`
}

func main() {
	// Fetch XML data
	resp, err := http.Get(endpoint)
	if err != nil {
		log.Fatalf("❌ Failed to fetch data: %v", err)
	}
	defer resp.Body.Close()

	xmlData, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("❌ Failed to read response: %v", err)
	}

	var events UpcomingEvents
	if err := xml.Unmarshal(xmlData, &events); err != nil {
		log.Fatalf("❌ Failed to parse XML: %v", err)
	}

	// Connect to MySQL
	dsn := "apps_user:Tb#<M#BnvBc%ur5q@tcp(10.79.224.2:3306)/kiron"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("❌ Failed to connect to database: %v", err)
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
			e.ID,
			e.EventType,
			e.EventNumber,
			e.EventTime.Time,
			e.FinishTime.Time,
			e.EventStatus,
			events.LocalTime.Time,
			events.UtcTime.Time,
			events.RoundTripTime.Time,
		)
		if err != nil {
			log.Printf("⚠️ Insert failed for ID %d: %v", e.ID, err)
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
