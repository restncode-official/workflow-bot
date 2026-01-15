package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(func(app core.App) error {
		log.Println("Running initialization migration...")

		// 1. Create 'projects' collection
		projectsProto := core.NewBaseCollection("projects")
		projectsProto.Fields.Add(&core.TextField{
			Name:     "channel_id",
			Required: true,
		})
		// Note: NewBaseCollection automatically creates 'id', 'created', 'updated'.
		// To enforce uniqueness on channel_id, we usually add an index or validator.
		// For simplicity in code definition, we just add the field.

		projectsProto.Fields.Add(&core.TextField{
			Name:     "name",
			Required: true,
		})

		// Save projects collection
		// Check if exists first to avoid error? Register implies it runs once per new setup usually.
		if err := app.Save(projectsProto); err != nil {
			return err
		}

		// 2. Create 'work_logs' collection
		logsProto := core.NewBaseCollection("work_logs")
		logsProto.Fields.Add(&core.TextField{
			Name:     "user_id",
			Required: true,
		})

		// Define relation to projects
		logsProto.Fields.Add(&core.RelationField{
			Name:          "project",
			CollectionId:  projectsProto.Id,
			Required:      true,
			MaxSelect:     1,
			CascadeDelete: false,
		})

		logsProto.Fields.Add(&core.DateField{
			Name:     "start_time",
			Required: true,
		})
		logsProto.Fields.Add(&core.DateField{
			Name: "end_time",
		}) // Nullable by default if not required

		return app.Save(logsProto)
	}, func(app core.App) error {
		// Revert logic (optional for this context)
		// app.Delete(projects) etc.
		return nil
	})
}
