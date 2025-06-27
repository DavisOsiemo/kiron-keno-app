package main

// type UpcomingEvents struct {
// 	XMLName       xml.Name    `xml:"UpcomingEvents"`
// 	LocalTime     time.Time   `xml:"LocalTime,attr"`
// 	UtcTime       time.Time   `xml:"UtcTime,attr"`
// 	RoundTripTime time.Time   `xml:"RoundTripTime,attr"`
// 	KenoEvents    []KenoEvent `xml:"KenoEvent"`
// }

// type KenoEvent struct {
// 	ID          int64     `xml:"ID,attr"`
// 	EventType   string    `xml:"EventType,attr"`
// 	EventNumber string    `xml:"EventNumber,attr"`
// 	EventTime   time.Time `xml:"EventTime,attr"`
// 	FinishTime  time.Time `xml:"FinishTime,attr"`
// 	EventStatus string    `xml:"EventStatus,attr"`
// }

// CREATE TABLE keno_events (
//     id BIGINT PRIMARY KEY,
//     event_type VARCHAR(20) NOT NULL,
//     event_number VARCHAR(20) NOT NULL,
//     event_time DATETIME NOT NULL,
//     finish_time DATETIME NOT NULL,
//     event_status VARCHAR(50) NOT NULL,
//     `local_time` DATETIME NOT NULL,
//     `utc_time` DATETIME NOT NULL,
//     `round_trip_time` DATETIME NOT NULL,
//     created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
// );
