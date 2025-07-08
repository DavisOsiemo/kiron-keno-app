package main

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

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

func getKenoEventsHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := db.Query(`
			SELECT id, event_number, keno_event_id, results, status_desc, status,
			       start_time_utc, end_time_utc FROM keno_events
			ORDER BY start_time_utc DESC
			LIMIT 100
		`)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query events"})
			return
		}
		defer rows.Close()

		var events []KenoEventRow
		for rows.Next() {
			var e KenoEventRow
			if err := rows.Scan(
				&e.ID, &e.EventNumber, &e.KenoEventID, &e.Results,
				&e.StatusDesc, &e.Status,
				&e.StartTimeUTC, &e.EndTimeUTC,
			); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse events"})
				return
			}
			events = append(events, e)
		}

		c.JSON(http.StatusOK, events)
	}
}
