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

// CustomTime handles timestamps with multiple possible formats
type CustomTime struct {
	time.Time
}

func (ct *CustomTime) UnmarshalXMLAttr(attr xml.Attr) error {
	formats := []string{
		"2006-01-02T15:04:05",       // e.g. 2025-06-27T13:03:00 (no timezone)
		"2006-01-02 15:04:05Z",      // e.g. 2025-06-27 13:02:47Z (UTC)
		"2006-01-02T15:04:05Z07:00", // e.g. 2025-06-27T13:03:00+01:00 (full TZ offset)
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

	// Log all events
	for _, e := range events.KenoEvents {
		log.Printf("Event ID: %d, Type: %s, Number: %s, EventTime: %s, FinishTime: %s, Status: %s",
			e.ID, e.EventType, e.EventNumber, e.EventTime.Format(time.RFC3339), e.FinishTime.Format(time.RFC3339), e.EventStatus)
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
