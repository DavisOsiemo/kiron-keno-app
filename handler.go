package main

import (
	"database/sql"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Final response structure (clean JSON output)
type KenoEventRow struct {
	ID           int64     `json:"id"`
	EventNumber  int64     `json:"event_number"`
	KenoEventID  int64     `json:"keno_event_id"`
	Results      string    `json:"results"`
	StatusDesc   string    `json:"status_desc"`
	Status       int       `json:"status"`
	StartTimeUTC time.Time `json:"start_time_utc"`
	EndTimeUTC   time.Time `json:"end_time_utc"`
}

// Handler function to fetch and return keno_events
func getKenoEventsHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := db.Query(`
			SELECT id, event_number, keno_event_id, results, status_desc, status, start_time_utc, end_time_utc
			FROM keno_events
			ORDER BY start_time_utc DESC
		`)
		if err != nil {
			log.Printf("❌ Query error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query events"})
			return
		}
		defer rows.Close()

		var events []KenoEventRow

		for rows.Next() {
			// Temporary struct to safely hold NULL values
			var temp struct {
				ID           int64
				EventNumber  int64
				KenoEventID  int64
				Results      sql.NullString
				StatusDesc   string
				Status       int
				StartTimeUTC time.Time
				EndTimeUTC   time.Time
			}

			if err := rows.Scan(
				&temp.ID, &temp.EventNumber, &temp.KenoEventID,
				&temp.Results, &temp.StatusDesc, &temp.Status,
				&temp.StartTimeUTC, &temp.EndTimeUTC,
			); err != nil {
				log.Printf("❌ Scan error: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse events"})
				return
			}

			// Convert to safe output struct
			event := KenoEventRow{
				ID:           temp.ID,
				EventNumber:  temp.EventNumber,
				KenoEventID:  temp.KenoEventID,
				Results:      nullToEmpty(temp.Results),
				StatusDesc:   temp.StatusDesc,
				Status:       temp.Status,
				StartTimeUTC: temp.StartTimeUTC,
				EndTimeUTC:   temp.EndTimeUTC,
			}

			events = append(events, event)
		}

		c.JSON(http.StatusOK, events)
	}
}

// Helper function to convert sql.NullString to string safely
func nullToEmpty(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

// package main

// import (
// 	"database/sql"
// 	"log"
// 	"net/http"
// 	"time"

// 	"github.com/gin-gonic/gin"
// )

// type KenoEventRow struct {
// 	ID           int64     `json:"id"`
// 	EventNumber  int64     `json:"event_number"`
// 	KenoEventID  int64     `json:"keno_event_id"`
// 	Results      string    `json:"results"`
// 	StatusDesc   string    `json:"status_desc"`
// 	Status       int       `json:"status"`
// 	StartTimeUTC time.Time `json:"start_time_utc"`
// 	EndTimeUTC   time.Time `json:"end_time_utc"`
// }

// func getKenoEventsHandler(db *sql.DB) gin.HandlerFunc {
// 	return func(c *gin.Context) {
// 		rows, err := db.Query(`SELECT id, event_number, keno_event_id, results, status_desc, status, start_time_utc, end_time_utc FROM keno_events ORDER BY start_time_utc DESC`)
// 		if err != nil {
// 			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query events"})
// 			return
// 		}
// 		defer rows.Close()

// 		var events []KenoEventRow
// 		for rows.Next() {
// 			var e KenoEventRow
// 			if err := rows.Scan(
// 				&e.ID, &e.EventNumber, &e.KenoEventID, &e.Results, &e.StatusDesc, &e.Status, &e.StartTimeUTC, &e.EndTimeUTC,
// 			); err != nil {
// 				log.Printf("❌ Scan error: %v", err)
// 				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse events"})
// 				return
// 			}
// 			events = append(events, e)
// 		}

// 		c.JSON(http.StatusOK, events)
// 	}
// }
