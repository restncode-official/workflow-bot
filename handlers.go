package main

import (
	"log"
	"time"

	"github.com/disgoorg/disgo/events"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

const (
	// Replace with your actual Break Channel ID
	BreakChannelID = "1460937344722145320"
)

// VoiceStateUpdateHandler closure
func VoiceStateUpdateHandler(app core.App) func(event *events.GuildVoiceStateUpdate) {
	return func(event *events.GuildVoiceStateUpdate) {
		userID := event.VoiceState.UserID.String()
		var newChannelID string
		if event.VoiceState.ChannelID != nil {
			newChannelID = event.VoiceState.ChannelID.String()
		}

		// 1. Check for any active log for this user
		activeLog, err := app.FindFirstRecordByFilter("work_logs",
			"user_id = {:uid} && end_time = ''",
			dbx.Params{"uid": userID},
		)

		// 2. If active log exists, check if we need to close it
		if err == nil && activeLog != nil {
			// Fetch the project associated with the active log
			projectID := activeLog.GetString("project")
			project, err := app.FindRecordById("projects", projectID)

			if err == nil && project != nil {
				currentProjectChannelID := project.GetString("channel_id")

				// If user is currently in the same channel as the active log, it's just a state update (mute/deaf)
				if newChannelID == currentProjectChannelID {
					return
				}
			}

			// User left the channel or moved to another. Close the active log.
			activeLog.Set("end_time", time.Now())
			if err := app.Save(activeLog); err != nil {
				log.Printf("Error closing log for user %s: %v", userID, err)
			} else {
                log.Printf("Closed log for user %s (Project: %s)", userID, projectID)
            }
		}

		// 3. If user joined a channel (newChannelID is set), try to start a new log
		if newChannelID != "" && newChannelID != BreakChannelID {
			if err := createNewLog(app, userID, newChannelID); err != nil {
				// Only log error if it's NOT a "record not found" (meaning not a project channel)
                // Actually createNewLog returns nil if not a project, so any error is real.
				log.Printf("Error creating log: %v", err)
			}
		}
	}
}

func createNewLog(app core.App, userID, channelID string) error {
	// Check if channel is project
	project, err := app.FindFirstRecordByFilter("projects", "channel_id = {:id}", dbx.Params{"id": channelID})
	if err != nil {
		return nil // Not a project
	}

	// Create log
	collection, err := app.FindCollectionByNameOrId("work_logs")
	if err != nil {
		return err // Should exist
	}

	record := core.NewRecord(collection)
	record.Set("user_id", userID)
	record.Set("project", project.Id)
	record.Set("start_time", time.Now())

	return app.Save(record)
}
