package api

import (
	"github.com/gin-gonic/gin"
	"github.com/snail2sky/starfleet/internal/api/handler"
	"github.com/snail2sky/starfleet/internal/api/middleware"
	"github.com/snail2sky/starfleet/internal/api/ws"
	"github.com/snail2sky/starfleet/internal/config"
	"github.com/snail2sky/starfleet/internal/engine"
	"github.com/snail2sky/starfleet/internal/scheduler"
)

// Router sets up all API routes.
type Router struct {
	engine *gin.Engine
}

// NewRouter creates and configures the API router.
func NewRouter(mgr *engine.Manager, sched *scheduler.Scheduler, hub *ws.Hub, cfg *config.Config, reloadFn func() error) *Router {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.CORS())
	r.Use(middleware.Logger())

	// Auth middleware.
	authCfg := middleware.AuthConfig{
		Enabled:  cfg.Server.Auth.Enabled,
		Username: cfg.Server.Auth.Username,
		Password: cfg.Server.Auth.Password,
		Secret:   cfg.Server.Auth.Secret,
	}

	// Handlers.
	processH := handler.NewProcessHandler(mgr)
	systemH := handler.NewSystemHandler()
	configH := handler.NewConfigHandler(cfg, reloadFn)
	scheduleH := handler.NewScheduleHandler(sched)

	// Health check (no auth required).
	r.GET("/health", systemH.Health)

	// API v1.
	v1 := r.Group("/api/v1")
	v1.Use(middleware.BasicAuth(authCfg))
	{
		// Processes.
		procs := v1.Group("/processes")
		{
			procs.GET("", processH.List)
			procs.GET("/:name", processH.Get)
			procs.POST("/:name/start", processH.Start)
			procs.POST("/:name/stop", processH.Stop)
			procs.POST("/:name/restart", processH.Restart)
			procs.GET("/:name/logs", processH.Logs)
		}

		// Aggregated logs across all processes.
		v1.GET("/logs", handler.AllLogs)

		// Schedules.
		schedules := v1.Group("/schedules")
		{
			schedules.GET("", scheduleH.List)
			schedules.GET("/:name", scheduleH.Get)
			schedules.POST("/:name/run", scheduleH.Run)
			schedules.GET("/:name/history", scheduleH.History)
		}

		// System.
		sys := v1.Group("/system")
		{
			sys.GET("/overview", systemH.Overview)
			sys.GET("/health", systemH.Health)
		}

		// WebSocket endpoints.
		v1.GET("/ws/logs", hub.HandleWebSocket(ws.MsgLog))
		v1.GET("/ws/events", hub.HandleWebSocket(ws.MsgEvent))

		// Config.
		cfgGroup := v1.Group("/config")
		{
			cfgGroup.GET("", configH.Get)
			cfgGroup.POST("/reload", configH.Reload)
		}
	}

	// Serve embedded Vue frontend (SPA with history-mode fallback).
	ServeStatic(r)

	return &Router{engine: r}
}

// Run starts the HTTP server.
func (r *Router) Run(addr string) error {
	return r.engine.Run(addr)
}