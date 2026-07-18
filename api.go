package main

import (
	"net/http"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

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
