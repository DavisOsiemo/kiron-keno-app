package main

import (
	"database/sql"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
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
	dsn := "apps_user:Tb#<M#BnvBc%ur5q@tcp(10.79.224.2:3306)/moss_play_b2b_keno"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("❌ Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("❌ DB connection test failed: %v", err)
	}

	loc, _ := time.LoadLocation("Africa/Nairobi") // UTC

	tickerUpcomingEvents := time.NewTicker(60 * time.Second)
	defer tickerUpcomingEvents.Stop()
	go func() {
		for {
			select {
			case <-tickerUpcomingEvents.C: // Wait for the ticker to tick
				log.Println("🔄 Running processUpcomingEvents")
				if err := processUpcomingEvents(db); err != nil {
					log.Printf("❌ Error processing UpcomingEvents: %v", err)
				}
			}
		}
	}()

	// Create a ticker for processKenoBallStats every 195 seconds (3 minutes and 15 seconds)
	tickerKenoBallStats := time.NewTicker(195 * time.Second)
	defer tickerKenoBallStats.Stop()
	go func() {
		for {
			select {
			case <-tickerKenoBallStats.C: // Wait for the ticker to tick
				log.Println("🔄 Running processKenoBallStats")
				if err := processKenoBallStats(db); err != nil {
					log.Printf("❌ Error processing KenoBallStats: %v", err)
				}
			}
		}
	}()

	// ✅ Ticker: processResults
	tickerResults := time.NewTicker(165 * time.Second)
	defer tickerResults.Stop()
	go func() {
		for range tickerResults.C {
			now := time.Now().In(loc)
			log.Printf("🔄 Running processResults for date %s", now.Format("2006-01-02"))
			if err := processResults(db, now); err != nil {
				log.Printf("❌ Error processing Results: %v", err)
			}
		}
	}()

	go func() {
		r := gin.Default()
		r.GET("/keno-events", getKenoEventsHandler(db)) // Your new endpoint
		log.Println("🚀 Starting Gin server on :8080")
		if err := r.Run(":8080"); err != nil {
			log.Fatalf("❌ Failed to run Gin server: %v", err)
		}
	}()

	select {}
}

func mapEventStatusToInt(status string) int {
	switch status {
	case "Pending":
		return 0
	case "InProgress", "OpenForBetting":
		return 1
	case "Finished":
		return 2
	case "BettingSuspended":
		return 3
	default:
		return 0
	}
}

func parseEventNumber(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
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

	insertStmt := `
	INSERT INTO keno_events (
		event_number, keno_event_id,
		results, status_desc, status,
		start_time_utc, end_time_utc,
		created
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	ON DUPLICATE KEY UPDATE
		results = VALUES(results),
		status_desc = VALUES(status_desc),
		status = VALUES(status),
		start_time_utc = VALUES(start_time_utc),
		end_time_utc = VALUES(end_time_utc),
		updated = CURRENT_TIMESTAMP
	`

	now := time.Now().UTC()

	for _, e := range events.KenoEvents {
		eventNumber, err := parseEventNumber(e.EventNumber)
		if err != nil {
			log.Printf("⚠️ Failed to parse EventNumber '%s': %v", e.EventNumber, err)
			continue
		}

		statusInt := mapEventStatusToInt(e.EventStatus)

		_, err = db.Exec(insertStmt,
			//e.ID,                    // event_id
			eventNumber,             // event_number
			e.ID,                    // keno_event_id
			e.Result,                // results
			e.EventStatus,           // status_desc
			statusInt,               // status (mapped)
			e.EventTime.Time.UTC(),  // start_time_utc
			e.FinishTime.Time.UTC(), // end_time_utc
			now,                     // created
		)

		// 0 Pending
		// 1 InProgress, OpenForBetting
		// 2 Finished
		// 3 Suspended

		if err != nil {
			log.Printf("⚠️ Insert failed for UpcomingEvent ID %d: %v", e.ID, err)
		} else {
			log.Printf("✅ Inserted/Updated UpcomingEvent ID %d", e.ID)
		}
	}

	log.Println("✅ UpcomingEvents data inserted into `keno_events` successfully.")
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

	// Insert into keno_standings
	insertStmt := `
	INSERT INTO keno_standings (
		event_number, game_id, event_time, draw, status, created, updated
	) VALUES (?, ?, ?, ?, ?, ?, ?)
	ON DUPLICATE KEY UPDATE
		event_number = VALUES(event_number),
		event_time = VALUES(event_time),
		draw = VALUES(draw),
		status = VALUES(status),
		updated = CURRENT_TIMESTAMP
	`

	// Set status to 1 (for example, if you're considering the status as "active" or "running")
	status := 1 // Modify as needed depending on the status of the game

	for _, g := range stats.LastGames {
		_, err := db.Exec(insertStmt,
			g.EventNumber, // event_number
			g.ID,          // game_id
			g.EventTime.Time.UTC().Format("2006-01-02 15:04:05"), // event_time
			g.Draw,               // draw
			status,               // status (can be modified based on game status)
			stats.LocalTime.Time, // created
			time.Now().UTC(),     // updated
		)
		if err != nil {
			log.Printf("⚠️ Insert failed for KenoBallStats Game ID %d: %v", g.ID, err)
		} else {
			log.Printf("✅ Inserted/Updated Game ID %d into keno_standings", g.ID)
		}
	}

	log.Println("✅ KenoBallStats LastGames data inserted successfully into keno_standings.")
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

	for _, e := range results.KenoEvents {
		log.Printf("Result KenoEvent - ID: %d, Number: %s, EventTime: %s, FinishTime: %s, Status: %s, DrawMode: %s, Result: %s",
			e.ID, e.EventNumber, e.EventTime.Format(time.RFC3339), e.FinishTime.Format(time.RFC3339),
			e.EventStatus, e.DrawMode, e.Result)

		// Update the result in keno_events
		updateStmt := `
		UPDATE keno_events
		SET results = ?
		WHERE keno_event_id = ?
	`
		_, err = db.Exec(updateStmt, e.Result, e.ID)
		if err != nil {
			log.Printf("⚠️ Failed to update keno_events with result for ID %d: %v", e.ID, err)
		} else {
			log.Printf("✅ Updated keno_events.result for keno_event_id = %d", e.ID)
		}
	}

	log.Println("✅ Results data inserted successfully.")
	return nil
}
