package api

import (
	"net/http"
	"time"

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
	engine       *gin.Engine
	readTimeout  time.Duration
	writeTimeout time.Duration
}

// NewRouter creates and configures the API router.
// auth is the live auth state (updates via config reload apply immediately);
// loadCfg lets GET /config read the config from disk instead of the startup
// snapshot. persistAutostart/persistAutorestart write process toggles back
// to config files; persistScheduleEnabled does the same for scheduled jobs
// (all optional).
func NewRouter(mgr *engine.Manager, sched *scheduler.Scheduler, hub *ws.Hub, cfg *config.Config, auth *middleware.AuthState, loadCfg func() (*config.Config, error), reloadFn func() error, persistAutostart func(name string, enabled bool) error, persistAutorestart func(name string, value string) error, persistSchedule func(name string, enabled bool) error) *Router {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	// Real client IPs only: never trust X-Forwarded-For from arbitrary
	// peers (gin trusts all proxies by default, which lets anyone spoof
	// their address in logs and any future per-IP controls).
	_ = r.SetTrustedProxies(nil)
	r.Use(gin.Recovery())
	r.Use(middleware.CORS())
	r.Use(middleware.Logger())

	// Handlers.
	processH := handler.NewProcessHandler(mgr)
	if persistAutostart != nil {
		processH.SetAutostartPersister(persistAutostart)
	}
	if persistAutorestart != nil {
		processH.SetAutorestartPersister(persistAutorestart)
	}
	systemH := handler.NewSystemHandler(auth)
	authH := handler.NewAuthHandler(auth)
	configH := handler.NewConfigHandler(cfg, loadCfg, reloadFn)
	scheduleH := handler.NewScheduleHandler(sched)
	if persistSchedule != nil {
		scheduleH.SetEnabledPersister(persistSchedule)
	}

	// Health check (no auth required).
	r.GET("/health", systemH.Health)

	// Login issues a token for the WebUI (no auth required by definition).
	r.POST("/api/v1/auth/login", authH.Login)

	// API v1.
	v1 := r.Group("/api/v1")
	v1.Use(middleware.Authenticate(auth))
	{
		// Processes.
		procs := v1.Group("/processes")
		{
			procs.GET("", processH.List)
			procs.GET("/:name", processH.Get)
			procs.POST("/:name/start", processH.Start)
			procs.POST("/:name/stop", processH.Stop)
			procs.POST("/:name/restart", processH.Restart)
			procs.POST("/:name/enable", processH.Enable)
			procs.POST("/:name/disable", processH.Disable)
			procs.POST("/:name/autorestart", processH.SetAutorestart)
			procs.GET("/:name/logs", processH.Logs)
		}

		// Aggregated logs across all processes.
		v1.GET("/logs", handler.AllLogs)

		// Starfleet's own server logs.
		v1.GET("/server/logs", handler.ServerLogs)

		// Schedules.
		schedules := v1.Group("/schedules")
		{
			schedules.GET("", scheduleH.List)
			schedules.GET("/:name", scheduleH.Get)
			schedules.POST("/:name/run", scheduleH.Run)
			schedules.GET("/:name/history", scheduleH.History)
			schedules.POST("/:name/enable", scheduleH.Enable)
			schedules.POST("/:name/disable", scheduleH.Disable)
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

	// Serve embedded Vue frontend (SPA with history-mode fallback) unless
	// disabled in config — headless deployments keep only the API.
	if cfg.Server.Web.Enabled {
		ServeStatic(r)
	}

	readTO := cfg.Server.ReadTimeout
	if readTO <= 0 {
		readTO = 30 * time.Second
	}
	writeTO := cfg.Server.WriteTimeout
	if writeTO <= 0 {
		writeTO = 30 * time.Second
	}
	return &Router{engine: r, readTimeout: readTO, writeTimeout: writeTO}
}

// Run starts the HTTP server with the configured timeouts. Without
// ReadTimeout a slowloris client (sending headers byte by byte) pins a
// goroutine forever and can exhaust the server.
func (r *Router) Run(addr string) error {
	srv := &http.Server{
		Addr:         addr,
		Handler:      r.engine,
		ReadTimeout:  r.readTimeout,
		WriteTimeout: r.writeTimeout,
		IdleTimeout:  120 * time.Second,
	}
	return srv.ListenAndServe()
}
