package main

import (
	"net/http"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

type ProjectInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ChannelID string `json:"channelId"`
}

type ChannelInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ProjectID string `json:"projectId,omitempty"`
}

type createProjectBody struct {
	Name      string `json:"name"`
	ChannelID string `json:"channelId"`
}

func registerWorkflowAPI(app core.App, e *core.ServeEvent) {
	api := e.Router.Group("/api/workflow")

	api.GET("/stats", func(e *core.RequestEvent) error {
		q := app.DB().Select(
			"projects.name as projectName",
			"SUM(strftime('%s', work_logs.end_time) - strftime('%s', work_logs.start_time)) as totalSeconds",
		).
			From("work_logs").
			Join("LEFT JOIN", "projects", dbx.NewExp("projects.id = work_logs.project")).
			Where(workLogFilter(e, "work_logs.")).
			GroupBy("projects.id")

		var stats []ProjectStat
		if err := q.All(&stats); err != nil {
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		if stats == nil {
			stats = []ProjectStat{}
		}
		return e.JSON(http.StatusOK, stats)
	})

	api.GET("/leaderboard", func(e *core.RequestEvent) error {
		q := app.DB().Select(
			"user_id",
			"SUM(strftime('%s', end_time) - strftime('%s', start_time)) as totalSeconds",
		).
			From("work_logs").
			Where(workLogFilter(e, "")).
			GroupBy("user_id").
			OrderBy("totalSeconds DESC").
			Limit(10)

		var stats []UserStat
		if err := q.All(&stats); err != nil {
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		if stats == nil {
			stats = []UserStat{}
		}
		fillUserNames(stats)
		return e.JSON(http.StatusOK, stats)
	})

	api.GET("/timeline", func(e *core.RequestEvent) error {
		q := app.DB().Select(
			"strftime('%Y-%m-%d', start_time) as date",
			"SUM(strftime('%s', end_time) - strftime('%s', start_time)) as totalSeconds",
		).
			From("work_logs").
			Where(workLogFilter(e, "")).
			GroupBy("date").
			OrderBy("date ASC")

		// Keep a short default window when no date filter is set
		if e.Request.URL.Query().Get("from") == "" && e.Request.URL.Query().Get("to") == "" {
			q = q.Limit(30)
		}

		var stats []DailyStat
		if err := q.All(&stats); err != nil {
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		if stats == nil {
			stats = []DailyStat{}
		}
		return e.JSON(http.StatusOK, stats)
	})

	api.GET("/users", func(e *core.RequestEvent) error {
		var stats []UserStat
		err := app.DB().Select("user_id").
			From("work_logs").
			Where(dbx.NewExp("end_time != ''")).
			GroupBy("user_id").
			OrderBy("user_id ASC").
			All(&stats)
		if err != nil {
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		if stats == nil {
			stats = []UserStat{}
		}
		fillUserNames(stats)
		return e.JSON(http.StatusOK, stats)
	})

	api.GET("/projects", func(e *core.RequestEvent) error {
		records, err := app.FindAllRecords("projects")
		if err != nil {
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		out := make([]ProjectInfo, 0, len(records))
		for _, r := range records {
			out = append(out, ProjectInfo{
				ID:        r.Id,
				Name:      r.GetString("name"),
				ChannelID: r.GetString("channel_id"),
			})
		}
		return e.JSON(http.StatusOK, out)
	})

	api.GET("/channels", func(e *core.RequestEvent) error {
		channels := listVoiceChannels()
		if channels == nil {
			channels = []ChannelInfo{}
		}

		projects, _ := app.FindAllRecords("projects")
		byChannel := map[string]string{}
		for _, p := range projects {
			byChannel[p.GetString("channel_id")] = p.Id
		}
		for i := range channels {
			channels[i].ProjectID = byChannel[channels[i].ID]
		}
		return e.JSON(http.StatusOK, channels)
	})

	// Writes require HTTP Basic Auth (WORKFLOW_ADMIN_PASSWORD)
	admin := api.Group("")
	admin.BindFunc(requireBasicAuth)

	admin.POST("/projects", func(e *core.RequestEvent) error {
		var body createProjectBody
		if err := e.BindBody(&body); err != nil {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		}
		body.Name = strings.TrimSpace(body.Name)
		body.ChannelID = strings.TrimSpace(body.ChannelID)
		if body.Name == "" || body.ChannelID == "" {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "name and channelId are required"})
		}
		if _, err := parseSnowflake(body.ChannelID); err != nil {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "channelId must be a valid Discord snowflake"})
		}

		existing, _ := app.FindFirstRecordByFilter("projects", "channel_id = {:id}", dbx.Params{"id": body.ChannelID})
		if existing != nil {
			return e.JSON(http.StatusConflict, map[string]string{"error": "a project already uses this channel"})
		}

		collection, err := app.FindCollectionByNameOrId("projects")
		if err != nil {
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		record := core.NewRecord(collection)
		record.Set("name", body.Name)
		record.Set("channel_id", body.ChannelID)
		if err := app.Save(record); err != nil {
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}

		return e.JSON(http.StatusCreated, ProjectInfo{
			ID:        record.Id,
			Name:      body.Name,
			ChannelID: body.ChannelID,
		})
	})

	admin.DELETE("/projects/{id}", func(e *core.RequestEvent) error {
		id := e.Request.PathValue("id")
		record, err := app.FindRecordById("projects", id)
		if err != nil {
			return e.JSON(http.StatusNotFound, map[string]string{"error": "project not found"})
		}
		if err := app.Delete(record); err != nil {
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return e.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})
}

func listVoiceChannels() []ChannelInfo {
	if discordClient == nil {
		return nil
	}

	seen := map[string]bool{}
	var out []ChannelInfo

	add := func(ch discord.GuildChannel) {
		if ch.Type() != discord.ChannelTypeGuildVoice && ch.Type() != discord.ChannelTypeGuildStageVoice {
			return
		}
		id := ch.ID().String()
		if seen[id] {
			return
		}
		seen[id] = true
		out = append(out, ChannelInfo{ID: id, Name: ch.Name()})
	}

	discordClient.Caches().ChannelsForEach(add)

	if len(out) == 0 {
		discordClient.Caches().GuildsForEach(func(g discord.Guild) {
			channels, err := discordClient.Rest().GetGuildChannels(g.ID)
			if err != nil {
				return
			}
			for _, ch := range channels {
				add(ch)
			}
		})
	}

	return out
}

// workLogFilter builds WHERE conditions from ?from=&to=&userId=
// tablePrefix should be "work_logs." when joining, otherwise "".
func workLogFilter(e *core.RequestEvent, tablePrefix string) dbx.Expression {
	q := e.Request.URL.Query()
	conds := []dbx.Expression{
		dbx.NewExp(tablePrefix + "end_time != ''"),
	}
	if from := strings.TrimSpace(q.Get("from")); from != "" {
		conds = append(conds, dbx.NewExp("date("+tablePrefix+"start_time) >= {:from}", dbx.Params{"from": from}))
	}
	if to := strings.TrimSpace(q.Get("to")); to != "" {
		conds = append(conds, dbx.NewExp("date("+tablePrefix+"start_time) <= {:to}", dbx.Params{"to": to}))
	}
	if userID := strings.TrimSpace(q.Get("userId")); userID != "" {
		conds = append(conds, dbx.NewExp(tablePrefix+"user_id = {:userId}", dbx.Params{"userId": userID}))
	}
	return dbx.And(conds...)
}

func fillUserNames(stats []UserStat) {
	for i := range stats {
		stats[i].UserName = resolveUserName(stats[i].UserID)
	}
}

func resolveUserName(userID string) string {
	if discordClient == nil {
		return userID
	}
	id, err := parseSnowflake(userID)
	if err != nil {
		return userID
	}
	user, err := discordClient.Rest().GetUser(id)
	if err != nil {
		return userID
	}
	return user.EffectiveName()
}
