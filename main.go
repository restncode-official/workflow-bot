package main

import (
	"context"
	"log"
	"os"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/gateway"
	"github.com/joho/godotenv"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"

	_ "workflow/migrations"
)

const (
	DiscordTokenEnv = "DISCORD_TOKEN"
)

// Global Discord client (set after bot starts)
var discordClient bot.Client

type ProjectStat struct {
	ProjectName  string  `json:"projectName" db:"projectName"`
	TotalSeconds float64 `json:"totalSeconds" db:"totalSeconds"`
}

type UserStat struct {
	UserID       string  `json:"userId" db:"user_id"`
	UserName     string  `json:"userName"`
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

		registerWorkflowAPI(app, e)

		token := os.Getenv(DiscordTokenEnv)
		if token == "" {
			log.Println("Warning: DISCORD_TOKEN is not set. Bot will not start.")
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

	// Store client globally for API access
	discordClient = client

	if err = client.OpenGateway(context.Background()); err != nil {
		log.Printf("Error connecting to gateway: %v", err)
		return
	}

	log.Println("Discord Bot started")
}
