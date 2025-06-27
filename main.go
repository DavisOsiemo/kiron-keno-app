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

const (
	upcomingEventsEndpoint = "http://vseintegration.kironinteractive.com:8013/vsegameserver/dataservice/UpcomingEvents?hours=4&type=Keno"
	kenoBallStatsEndpoint  = "http://vseintegration.kironinteractive.com:8013/vsegameserver/dataservice/KenoBallStats"
	resultsEndpointFormat  = "http://vseintegration.kironinteractive.com:8013/vsegameserver/dataservice/Results/%04d/%02d/%02d?type=Keno"
)

// CustomTime handles multiple time formats in XML attributes
type CustomTime struct {
	time.Time
}

func (ct *CustomTime) UnmarshalXMLAttr(attr xml.Attr) error {
	formats := []string{
		"2006-01-02T15:04:05",               // no timezone
		"2006-01-02 15:04:05Z",              // UTC
		"2006-01-02T15:04:05Z07:00",         // full TZ offset
		"2006-01-02T15:04:05.9999999Z",      // fractional seconds UTC
		"2006-01-02T15:04:05.9999999Z07:00", // fractional seconds with TZ
	}

	var lastErr error
	for _, layout := range formats {
		t, err := time.Parse(layout, attr.Value)
		if err == nil {
			ct.Time = t
			return nil
		}
		lastErr = err
	}
	return lastErr
}

// UpcomingEvents structs
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
	DrawMode    string     `xml:"DrawMode,attr,omitempty"` // optional attribute in Results
	Result      string     `xml:"Result,attr,omitempty"`   // optional attribute in Results
}

// KenoBallStats structs
type KenoBallStats struct {
	XMLName       xml.Name    `xml:"KenoBallStats"`
	LocalTime     CustomTime  `xml:"LocalTime,attr"`
	UtcTime       CustomTime  `xml:"UtcTime,attr"`
	RoundTripTime CustomTime  `xml:"RoundTripTime,attr"`
	LastGames     []Game      `xml:"LastGames>Game"`
	HotBalls      []BallStats `xml:"Hot>Ball"`
	ColdBalls     []BallStats `xml:"Cold>Ball"`
	Hits          []BallStats `xml:"Hits>Ball"`
}

type Game struct {
	ID          int64      `xml:"ID,attr"`
	EventNumber string     `xml:"EventNumber,attr"`
	EventTime   CustomTime `xml:"EventTime,attr"`
	Draw        string     `xml:"Draw,attr"`
}

type BallStats struct {
	Number int `xml:"Number,attr"`
	Hits   int `xml:"Hits,attr"`
}

// Results structs
type Results struct {
	XMLName       xml.Name    `xml:"Results"`
	LocalTime     CustomTime  `xml:"LocalTime,attr"`
	UtcTime       CustomTime  `xml:"UtcTime,attr"`
	RoundTripTime CustomTime  `xml:"RoundTripTime,attr"`
	KenoEvents    []KenoEvent `xml:"KenoEvent"`
}

func main() {
	// Connect to DB once
	dsn := "apps_user:Tb#<M#BnvBc%ur5q@tcp(10.79.224.2:3306)/kiron"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("❌ Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Process UpcomingEvents
	if err := processUpcomingEvents(db); err != nil {
		log.Printf("❌ Error processing UpcomingEvents: %v", err)
	}

	// Process KenoBallStats
	if err := processKenoBallStats(db); err != nil {
		log.Printf("❌ Error processing KenoBallStats: %v", err)
	}

	// Process Results for 2025-06-26 (example date, change as needed)
	targetDate := time.Date(2025, 6, 26, 0, 0, 0, 0, time.UTC)
	if err := processResults(db, targetDate); err != nil {
		log.Printf("❌ Error processing Results: %v", err)
	}
}

func processUpcomingEvents(db *sql.DB) error {
	resp, err := http.Get(upcomingEventsEndpoint)
	if err != nil {
		return fmt.Errorf("failed to fetch UpcomingEvents: %w", err)
	}
	defer resp.Body.Close()

	xmlData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read UpcomingEvents response: %w", err)
	}

	var events UpcomingEvents
	if err := xml.Unmarshal(xmlData, &events); err != nil {
		return fmt.Errorf("failed to unmarshal UpcomingEvents XML: %w", err)
	}

	// Log events
	for _, e := range events.KenoEvents {
		log.Printf("UpcomingEvent - ID: %d, Type: %s, Number: %s, EventTime: %s, FinishTime: %s, Status: %s",
			e.ID, e.EventType, e.EventNumber, e.EventTime.Format(time.RFC3339), e.FinishTime.Format(time.RFC3339), e.EventStatus)
	}

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
			log.Printf("⚠️ Insert failed for UpcomingEvent ID %d: %v", e.ID, err)
		}
	}

	log.Println("✅ UpcomingEvents data inserted successfully.")
	return nil
}

func processKenoBallStats(db *sql.DB) error {
	resp, err := http.Get(kenoBallStatsEndpoint)
	if err != nil {
		return fmt.Errorf("failed to fetch KenoBallStats: %w", err)
	}
	defer resp.Body.Close()

	xmlData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read KenoBallStats response: %w", err)
	}

	var stats KenoBallStats
	if err := xml.Unmarshal(xmlData, &stats); err != nil {
		return fmt.Errorf("failed to unmarshal KenoBallStats XML: %w", err)
	}

	// Log
	log.Printf("KenoBallStats - LocalTime: %s, UtcTime: %s, RoundTripTime: %s",
		stats.LocalTime.Format(time.RFC3339),
		stats.UtcTime.Format(time.RFC3339),
		stats.RoundTripTime.Format(time.RFC3339),
	)

	for _, g := range stats.LastGames {
		log.Printf("Game - ID: %d, EventNumber: %s, EventTime: %s, Draw: %s",
			g.ID, g.EventNumber, g.EventTime.Format(time.RFC3339), g.Draw)
	}

	// Insert LastGames into DB
	insertGameStmt := `
	INSERT INTO keno_ball_stats_games (
		id, event_number, event_time, draw, local_time, utc_time, round_trip_time
	) VALUES (?, ?, ?, ?, ?, ?, ?)
	ON DUPLICATE KEY UPDATE
		event_number=VALUES(event_number),
		event_time=VALUES(event_time),
		draw=VALUES(draw),
		local_time=VALUES(local_time),
		utc_time=VALUES(utc_time),
		round_trip_time=VALUES(round_trip_time)
	`

	for _, g := range stats.LastGames {
		_, err := db.Exec(insertGameStmt,
			g.ID,
			g.EventNumber,
			g.EventTime.Time,
			g.Draw,
			stats.LocalTime.Time,
			stats.UtcTime.Time,
			stats.RoundTripTime.Time,
		)
		if err != nil {
			log.Printf("⚠️ Insert failed for KenoBallStats Game ID %d: %v", g.ID, err)
		}
	}

	log.Println("✅ KenoBallStats LastGames data inserted successfully.")
	return nil
}

func processResults(db *sql.DB, date time.Time) error {
	url := fmt.Sprintf(resultsEndpointFormat, date.Year(), int(date.Month()), date.Day())
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to fetch Results: %w", err)
	}
	defer resp.Body.Close()

	xmlData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read Results response: %w", err)
	}

	var results Results
	if err := xml.Unmarshal(xmlData, &results); err != nil {
		return fmt.Errorf("failed to unmarshal Results XML: %w", err)
	}

	// Log each KenoEvent in Results
	for _, e := range results.KenoEvents {
		log.Printf("Result KenoEvent - ID: %d, Number: %s, EventTime: %s, FinishTime: %s, Status: %s, DrawMode: %s, Result: %s",
			e.ID, e.EventNumber, e.EventTime.Format(time.RFC3339), e.FinishTime.Format(time.RFC3339),
			e.EventStatus, e.DrawMode, e.Result)
	}

	insertStmt := `
	INSERT INTO keno_results (
		id, event_type, event_number, event_time, finish_time, event_status, draw_mode, result,
		local_time, utc_time, round_trip_time
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON DUPLICATE KEY UPDATE
		event_type=VALUES(event_type),
		event_number=VALUES(event_number),
		event_time=VALUES(event_time),
		finish_time=VALUES(finish_time),
		event_status=VALUES(event_status),
		draw_mode=VALUES(draw_mode),
		result=VALUES(result),
		local_time=VALUES(local_time),
		utc_time=VALUES(utc_time),
		round_trip_time=VALUES(round_trip_time)
	`

	for _, e := range results.KenoEvents {
		_, err := db.Exec(insertStmt,
			e.ID,
			e.EventType,
			e.EventNumber,
			e.EventTime.Time,
			e.FinishTime.Time,
			e.EventStatus,
			e.DrawMode,
			e.Result,
			results.LocalTime.Time,
			results.UtcTime.Time,
			results.RoundTripTime.Time,
		)
		if err != nil {
			log.Printf("⚠️ Insert failed for Result KenoEvent ID %d: %v", e.ID, err)
		}
	}

	log.Println("✅ Results data inserted successfully.")
	return nil
}

// package main

// import (
// 	"database/sql"
// 	"encoding/xml"
// 	"fmt"
// 	"io"
// 	"log"
// 	"net/http"
// 	"time"

// 	_ "github.com/go-sql-driver/mysql"
// 	"github.com/robfig/cron/v3"
// )

// const endpoint = "http://vseintegration.kironinteractive.com:8013/vsegameserver/dataservice/UpcomingEvents?hours=4&type=Keno"

// // CustomTime handles timestamps with multiple possible formats
// type CustomTime struct {
// 	time.Time
// }

// func (ct *CustomTime) UnmarshalXMLAttr(attr xml.Attr) error {
// 	formats := []string{
// 		"2006-01-02T15:04:05",       // e.g. 2025-06-27T13:03:00 (no timezone)
// 		"2006-01-02 15:04:05Z",      // e.g. 2025-06-27 13:02:47Z (UTC)
// 		"2006-01-02T15:04:05Z07:00", // e.g. 2025-06-27T13:03:00+01:00 (full TZ offset)
// 	}

// 	var lastErr error
// 	for _, layout := range formats {
// 		t, err := time.Parse(layout, attr.Value)
// 		if err == nil {
// 			ct.Time = t
// 			return nil
// 		}
// 		lastErr = err
// 	}
// 	return lastErr
// }

// type UpcomingEvents struct {
// 	XMLName       xml.Name    `xml:"UpcomingEvents"`
// 	LocalTime     CustomTime  `xml:"LocalTime,attr"`
// 	UtcTime       CustomTime  `xml:"UtcTime,attr"`
// 	RoundTripTime CustomTime  `xml:"RoundTripTime,attr"`
// 	KenoEvents    []KenoEvent `xml:"KenoEvent"`
// }

// type KenoEvent struct {
// 	ID          int64      `xml:"ID,attr"`
// 	EventType   string     `xml:"EventType,attr"`
// 	EventNumber string     `xml:"EventNumber,attr"`
// 	EventTime   CustomTime `xml:"EventTime,attr"`
// 	FinishTime  CustomTime `xml:"FinishTime,attr"`
// 	EventStatus string     `xml:"EventStatus,attr"`
// }

// func main() {
// 	// Fetch XML data
// 	resp, err := http.Get(endpoint)
// 	if err != nil {
// 		log.Fatalf("❌ Failed to fetch data: %v", err)
// 	}
// 	defer resp.Body.Close()

// 	xmlData, err := io.ReadAll(resp.Body)
// 	if err != nil {
// 		log.Fatalf("❌ Failed to read response: %v", err)
// 	}

// 	var events UpcomingEvents
// 	if err := xml.Unmarshal(xmlData, &events); err != nil {
// 		log.Fatalf("❌ Failed to parse XML: %v", err)
// 	}

// 	// Log all events
// 	for _, e := range events.KenoEvents {
// 		log.Printf("Event ID: %d, Type: %s, Number: %s, EventTime: %s, FinishTime: %s, Status: %s",
// 			e.ID, e.EventType, e.EventNumber, e.EventTime.Format(time.RFC3339), e.FinishTime.Format(time.RFC3339), e.EventStatus)
// 	}

// 	// Connect to MySQL
// 	dsn := "apps_user:Tb#<M#BnvBc%ur5q@tcp(10.79.224.2:3306)/kiron"
// 	db, err := sql.Open("mysql", dsn)
// 	if err != nil {
// 		log.Fatalf("❌ Failed to connect to database: %v", err)
// 	}
// 	defer db.Close()

// 	insertStmt := `
// 	INSERT INTO keno_events (
// 		id, event_type, event_number, event_time, finish_time, event_status,
// 		local_time, utc_time, round_trip_time
// 	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
// 	ON DUPLICATE KEY UPDATE
// 		event_type=VALUES(event_type),
// 		event_number=VALUES(event_number),
// 		event_time=VALUES(event_time),
// 		finish_time=VALUES(finish_time),
// 		event_status=VALUES(event_status),
// 		local_time=VALUES(local_time),
// 		utc_time=VALUES(utc_time),
// 		round_trip_time=VALUES(round_trip_time)
// 	`

// 	for _, e := range events.KenoEvents {
// 		_, err := db.Exec(insertStmt,
// 			e.ID,
// 			e.EventType,
// 			e.EventNumber,
// 			e.EventTime.Time,
// 			e.FinishTime.Time,
// 			e.EventStatus,
// 			events.LocalTime.Time,
// 			events.UtcTime.Time,
// 			events.RoundTripTime.Time,
// 		)
// 		if err != nil {
// 			log.Printf("⚠️ Insert failed for ID %d: %v", e.ID, err)
// 		}
// 	}

// 	fmt.Println("✅ Data inserted successfully.")
// }

func RunCron() {

	loc, _ := time.LoadLocation("Africa/Nairobi")
	c := cron.New(cron.WithLocation(loc))

	//c.AddFunc("0 4 * * *", ArchiveOddsLive)

	c.Start()
}
