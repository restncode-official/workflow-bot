package main

import (
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// CalculateTotalProjectTime calculates total duration for a project across all users (in seconds)
func CalculateTotalProjectTime(app core.App, projectID string) (float64, error) {
	var result struct {
		TotalSeconds float64 `db:"total_seconds"`
	}

	// Calculate difference between start and end time
	// Note: SQLite specific function unixepoch might need newer sqlite version.
	// Fallback: (strftime('%s', end_time) - strftime('%s', start_time))
	err := app.DB().Select("SUM(strftime('%s', end_time) - strftime('%s', start_time)) as total_seconds").
		From("work_logs").
		Where(dbx.HashExp{"project": projectID}).
		AndWhere(dbx.NewExp("end_time != ''")).
		One(&result)

	return result.TotalSeconds, err
}

// CalculateTotalUserTime calculates total time for a user across all projects (in seconds)
func CalculateTotalUserTime(app core.App, userID string) (float64, error) {
	var result struct {
		TotalSeconds float64 `db:"total_seconds"`
	}

	err := app.DB().Select("SUM(strftime('%s', end_time) - strftime('%s', start_time)) as total_seconds").
		From("work_logs").
		Where(dbx.HashExp{"user_id": userID}).
		AndWhere(dbx.NewExp("end_time != ''")).
		One(&result)

	return result.TotalSeconds, err
}
