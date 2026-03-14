package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zlc-ai/opc-platform/pkg/controller"
	"github.com/zlc-ai/opc-platform/pkg/cost"
	"github.com/zlc-ai/opc-platform/pkg/gateway"
	"go.uber.org/zap"
)

// Server provides HTTP API and WebSocket endpoints for the dashboard.
type Server struct {
	controller *controller.Controller
	costMgr    *cost.Tracker
	gateway    *gateway.Gateway
	logger     *zap.SugaredLogger
	httpServer *http.Server
	config     Config
}

// Config holds server configuration.
type Config struct {
	Port         int    `yaml:"port" json:"port"`
	Host         string `yaml:"host" json:"host"`
	CORSOrigins  []string `yaml:"corsOrigins" json:"corsOrigins"`
	EnableSwagger bool   `yaml:"enableSwagger" json:"enableSwagger"`
}

// New creates a new API server.
func New(
	ctrl *controller.Controller,
	costMgr *cost.Tracker,
	gw *gateway.Gateway,
	config Config,
	logger *zap.SugaredLogger,
) *Server {
	if config.Port == 0 {
		config.Port = 8080
	}
	if config.Host == "" {
		config.Host = "0.0.0.0"
	}

	return &Server{
		controller: ctrl,
		costMgr:    costMgr,
		gateway:    gw,
		logger:     logger,
		config:     config,
	}
}

// Start starts the HTTP server.
func (s *Server) Start(ctx context.Context) error {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(s.loggerMiddleware())
	router.Use(s.corsMiddleware())

	// API routes
	api := router.Group("/api")
	{
		// Agent endpoints
		api.GET("/agents", s.listAgents)
		api.GET("/agents/:name", s.getAgent)
		api.POST("/agents/:name/restart", s.restartAgent)

		// Task endpoints
		api.GET("/tasks", s.listTasks)
		api.GET("/tasks/:id", s.getTask)
		api.GET("/tasks/:id/logs", s.getTaskLogs)

		// Metrics endpoint
		api.GET("/metrics", s.getMetrics)

		// Cost endpoints
		api.GET("/costs/daily", s.getDailyCosts)
		api.GET("/costs/events", s.getCostEvents)

		// Workflow endpoints
		api.GET("/workflows", s.listWorkflows)

		// Logs endpoint
		api.GET("/logs", s.getLogs)

		// Health check
		api.GET("/health", s.healthCheck)
	}

	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: router,
	}

	s.logger.Infow("starting API server", "addr", addr)

	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Errorw("API server error", "error", err)
		}
	}()

	return nil
}

// Stop gracefully stops the HTTP server.
func (s *Server) Stop(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}

	s.logger.Info("stopping API server")
	return s.httpServer.Shutdown(ctx)
}

// Middleware

func (s *Server) loggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		s.logger.Debugw("HTTP request",
			"method", c.Request.Method,
			"path", path,
			"status", status,
			"latency", latency,
		)
	}
}

func (s *Server) corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		allowed := false

		if len(s.config.CORSOrigins) == 0 {
			allowed = true
		} else {
			for _, o := range s.config.CORSOrigins {
				if o == "*" || o == origin {
					allowed = true
					break
				}
			}
		}

		if allowed {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
			c.Header("Access-Control-Max-Age", "86400")
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// Handlers

func (s *Server) listAgents(c *gin.Context) {
	ctx := c.Request.Context()
	agents, err := s.controller.ListAgents(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Enrich with metrics
	allMetrics := s.controller.AgentMetrics()
	result := make([]gin.H, 0, len(agents))
	for _, a := range agents {
		item := gin.H{
			"name":      a.Name,
			"type":      a.Type,
			"phase":     a.Phase,
			"restarts":  a.Restarts,
			"message":   a.Message,
			"createdAt": a.CreatedAt,
			"updatedAt": a.UpdatedAt,
		}
		if m, ok := allMetrics[a.Name]; ok {
			item["metrics"] = m
		}
		result = append(result, item)
	}

	c.JSON(http.StatusOK, result)
}

func (s *Server) getAgent(c *gin.Context) {
	name := c.Param("name")
	ctx := c.Request.Context()

	agent, err := s.controller.GetAgent(ctx, name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}

	metrics := s.controller.AgentMetrics()
	result := gin.H{
		"name":      agent.Name,
		"type":      agent.Type,
		"phase":     agent.Phase,
		"restarts":  agent.Restarts,
		"message":   agent.Message,
		"createdAt": agent.CreatedAt,
		"updatedAt": agent.UpdatedAt,
	}
	if m, ok := metrics[agent.Name]; ok {
		result["metrics"] = m
	}

	c.JSON(http.StatusOK, result)
}

func (s *Server) restartAgent(c *gin.Context) {
	name := c.Param("name")
	ctx := c.Request.Context()

	if err := s.controller.RestartAgent(ctx, name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "restart initiated"})
}

func (s *Server) listTasks(c *gin.Context) {
	ctx := c.Request.Context()
	tasks, err := s.controller.Store().ListTasks(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tasks)
}

func (s *Server) getTask(c *gin.Context) {
	id := c.Param("id")
	ctx := c.Request.Context()

	task, err := s.controller.Store().GetTask(ctx, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}
	c.JSON(http.StatusOK, task)
}

func (s *Server) getTaskLogs(c *gin.Context) {
	id := c.Param("id")
	ctx := c.Request.Context()

	task, err := s.controller.Store().GetTask(ctx, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"logs": task.Result})
}

func (s *Server) getMetrics(c *gin.Context) {
	ctx := c.Request.Context()

	agents, _ := s.controller.ListAgents(ctx)
	tasks, _ := s.controller.Store().ListTasks(ctx)
	agentMetrics := s.controller.AgentMetrics()

	runningAgents := 0
	var totalCost float64
	for _, a := range agents {
		if a.Phase == "Running" {
			runningAgents++
		}
		if m, ok := agentMetrics[a.Name]; ok {
			totalCost += m.TotalCost
		}
	}

	runningTasks := 0
	completedTasks := 0
	failedTasks := 0
	for _, t := range tasks {
		switch t.Status {
		case "Running":
			runningTasks++
		case "Completed":
			completedTasks++
		case "Failed":
			failedTasks++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"totalAgents":    len(agents),
		"runningAgents":  runningAgents,
		"totalTasks":     len(tasks),
		"runningTasks":   runningTasks,
		"completedTasks": completedTasks,
		"failedTasks":    failedTasks,
		"todayCost":      totalCost,
		"monthCost":      totalCost * 1.5, // Placeholder
		"dailyBudget":    10.0,
		"monthlyBudget":  200.0,
	})
}

func (s *Server) getDailyCosts(c *gin.Context) {
	// Return mock data for now
	now := time.Now()
	data := make([]gin.H, 7)
	for i := 6; i >= 0; i-- {
		date := now.AddDate(0, 0, -i)
		data[6-i] = gin.H{
			"date": date.Format("2006-01-02"),
			"cost": float64(i%5+1) + float64(i%3)*0.5,
		}
	}
	c.JSON(http.StatusOK, data)
}

func (s *Server) getCostEvents(c *gin.Context) {
	c.JSON(http.StatusOK, []gin.H{})
}

func (s *Server) listWorkflows(c *gin.Context) {
	c.JSON(http.StatusOK, []gin.H{})
}

func (s *Server) getLogs(c *gin.Context) {
	c.JSON(http.StatusOK, []gin.H{})
}

func (s *Server) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
