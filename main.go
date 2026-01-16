package main

import (
	"context"
	"log"
	"os"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/gateway"
	"github.com/joho/godotenv"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"

	_ "workflow/migrations"
)

const (
	DiscordTokenEnv = "DISCORD_TOKEN"
)

type ProjectStat struct {
	ProjectName  string  `json:"projectName" db:"projectName"`
	TotalSeconds float64 `json:"totalSeconds" db:"totalSeconds"`
}

type UserStat struct {
	UserID       string  `json:"userId" db:"user_id"`
	TotalSeconds float64 `json:"totalSeconds" db:"totalSeconds"`
}

type DailyStat struct {
	Date         string  `json:"date" db:"date"`
	TotalSeconds float64 `json:"totalSeconds" db:"totalSeconds"`
}

func main() {
	// Load .env file if it exists
	_ = godotenv.Load()

	app := pocketbase.New()

	// Register migrations
	// automigrate: true allows creating migration files from UI changes automatically
	migratecmd.MustRegister(app, app.RootCmd, migratecmd.Config{
		Automigrate: true,
	})

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		// Serves static files from the provided public dir (if exists)
		e.Router.GET("/{path...}", apis.Static(os.DirFS("./pb_public"), false))

		// API Group
		api := e.Router.Group("/api/workflow")

		// 1. Project Stats
		api.GET("/stats", func(e *core.RequestEvent) error {
			var stats []ProjectStat
			err := app.DB().Select(
				"projects.name as projectName",
				"SUM(strftime('%s', work_logs.end_time) - strftime('%s', work_logs.start_time)) as totalSeconds",
			).
				From("work_logs").
				Join("LEFT JOIN", "projects", dbx.NewExp("projects.id = work_logs.project")).
				Where(dbx.NewExp("work_logs.end_time != ''")).
				GroupBy("projects.id").
				All(&stats)

			if err != nil {
				return e.JSON(500, map[string]string{"error": err.Error()})
			}
			return e.JSON(200, stats)
		})

		// 2. Leaderboard (Top Users)
		api.GET("/leaderboard", func(e *core.RequestEvent) error {
			var stats []UserStat
			err := app.DB().Select(
				"user_id",
				"SUM(strftime('%s', end_time) - strftime('%s', start_time)) as totalSeconds",
			).
				From("work_logs").
				Where(dbx.NewExp("end_time != ''")).
				GroupBy("user_id").
				OrderBy("totalSeconds DESC").
				Limit(10).
				All(&stats)

			if err != nil {
				return e.JSON(500, map[string]string{"error": err.Error()})
			}
			return e.JSON(200, stats)
		})

		// 3. Timeline (Daily Activity)
		api.GET("/timeline", func(e *core.RequestEvent) error {
			var stats []DailyStat
			// SQLite numeric date: strftime('%Y-%m-%d', start_time)
			err := app.DB().Select(
				"strftime('%Y-%m-%d', start_time) as date",
				"SUM(strftime('%s', end_time) - strftime('%s', start_time)) as totalSeconds",
			).
				From("work_logs").
				Where(dbx.NewExp("end_time != ''")).
				GroupBy("date").
				OrderBy("date ASC").
				Limit(30).
				All(&stats)

			if err != nil {
				return e.JSON(500, map[string]string{"error": err.Error()})
			}
			return e.JSON(200, stats)
		})

		token := os.Getenv(DiscordTokenEnv)
		if token == "" {
			log.Println("Warning: DISCORD_TOKEN is not set. Bot will not start.")
			// Continue serving PocketBase
			return e.Next()
		}

		go startDiscordBot(app, token)
		return e.Next()
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}

func startDiscordBot(app core.App, token string) {
	client, err := disgo.New(token,
		bot.WithGatewayConfigOpts(
			gateway.WithIntents(
				gateway.IntentGuildVoiceStates,
				gateway.IntentGuilds,
			),
		),
		bot.WithEventListenerFunc(VoiceStateUpdateHandler(app)),
	)
	if err != nil {
		log.Printf("Error creating Discord client: %v", err)
		return
	}

	if err = client.OpenGateway(context.Background()); err != nil {
		log.Printf("Error connecting to gateway: %v", err)
		return
	}

	log.Println("Discord Bot started")
}
