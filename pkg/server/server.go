package server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	v1 "github.com/zlc-ai/opc-platform/api/v1"
	"github.com/zlc-ai/opc-platform/pkg/controller"
	"github.com/zlc-ai/opc-platform/pkg/cost"
	"github.com/zlc-ai/opc-platform/pkg/gateway"
	"github.com/zlc-ai/opc-platform/pkg/workflow"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// Server provides HTTP API endpoints for the OPC daemon.
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
	Port          int      `yaml:"port" json:"port"`
	Host          string   `yaml:"host" json:"host"`
	CORSOrigins   []string `yaml:"corsOrigins" json:"corsOrigins"`
	EnableSwagger bool     `yaml:"enableSwagger" json:"enableSwagger"`
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
		config.Port = 9527
	}
	if config.Host == "" {
		config.Host = "127.0.0.1"
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

	api := router.Group("/api")
	{
		// --- daemon lifecycle ---
		api.GET("/health", s.healthCheck)
		api.GET("/status", s.clusterStatus)

		// --- apply (YAML in body) ---
		api.POST("/apply", s.applyResource)

		// --- agents ---
		api.GET("/agents", s.listAgents)
		api.GET("/agents/:name", s.getAgent)
		api.DELETE("/agents/:name", s.deleteAgent)
		api.POST("/agents/:name/start", s.startAgent)
		api.POST("/agents/:name/stop", s.stopAgent)
		api.POST("/agents/:name/restart", s.restartAgent)

		// --- tasks ---
		api.POST("/run", s.runTask)
		api.GET("/tasks", s.listTasks)
		api.GET("/tasks/:id", s.getTask)

		// --- metrics ---
		api.GET("/metrics/agents", s.agentMetrics)
		api.GET("/metrics/health", s.agentHealth)

		// --- workflows ---
		api.GET("/workflows", s.listWorkflows)
		api.DELETE("/workflows/:name", s.deleteWorkflow)
		api.POST("/workflows/:name/run", s.runWorkflow)
	}

	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: router,
	}

	s.logger.Infow("starting daemon", "addr", addr)

	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Errorw("daemon server error", "error", err)
		}
	}()

	return nil
}

// Stop gracefully stops the HTTP server.
func (s *Server) Stop(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}
	s.logger.Info("stopping daemon")
	return s.httpServer.Shutdown(ctx)
}

// ---- middleware ----

func (s *Server) loggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		c.Next()
		s.logger.Debugw("HTTP",
			"method", c.Request.Method,
			"path", path,
			"status", c.Writer.Status(),
			"latency", time.Since(start),
		)
	}
}

func (s *Server) corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// ---- handlers ----

func (s *Server) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

func (s *Server) clusterStatus(c *gin.Context) {
	ctx := c.Request.Context()
	agents, _ := s.controller.ListAgents(ctx)
	tasks, _ := s.controller.Store().ListTasks(ctx)

	var running, stopped, failed int
	for _, a := range agents {
		switch a.Phase {
		case v1.AgentPhaseRunning:
			running++
		case v1.AgentPhaseStopped:
			stopped++
		case v1.AgentPhaseFailed:
			failed++
		}
	}

	var pending, taskRunning, completed, taskFailed int
	for _, t := range tasks {
		switch t.Status {
		case v1.TaskStatusPending:
			pending++
		case v1.TaskStatusRunning:
			taskRunning++
		case v1.TaskStatusCompleted:
			completed++
		case v1.TaskStatusFailed:
			taskFailed++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"agents":         gin.H{"total": len(agents), "running": running, "stopped": stopped, "failed": failed},
		"tasks":          gin.H{"total": len(tasks), "pending": pending, "running": taskRunning, "completed": completed, "failed": taskFailed},
		"agentRecords":   agents,
	})
}

// ---- apply ----

func (s *Server) applyResource(c *gin.Context) {
	data, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "read body: " + err.Error()})
		return
	}

	var res v1.Resource
	if err := yaml.Unmarshal(data, &res); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "parse YAML: " + err.Error()})
		return
	}

	ctx := c.Request.Context()

	switch res.Kind {
	case v1.KindAgentSpec:
		var spec v1.AgentSpec
		if err := yaml.Unmarshal(data, &spec); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "parse AgentSpec: " + err.Error()})
			return
		}
		if err := s.controller.Apply(ctx, spec); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("agent/%s configured", spec.Metadata.Name)})

	case v1.KindWorkflow:
		wfSpec, err := workflow.ParseWorkflow(data)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		record := v1.WorkflowRecord{
			Name: wfSpec.Metadata.Name, SpecYAML: string(data),
			Schedule: wfSpec.Spec.Schedule, Enabled: true,
		}
		existing, getErr := s.controller.Store().GetWorkflow(ctx, wfSpec.Metadata.Name)
		if getErr == nil {
			existing.SpecYAML = string(data)
			existing.Schedule = wfSpec.Spec.Schedule
			if err := s.controller.Store().UpdateWorkflow(ctx, existing); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("workflow/%s updated", wfSpec.Metadata.Name)})
		} else {
			if err := s.controller.Store().CreateWorkflow(ctx, record); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("workflow/%s created", wfSpec.Metadata.Name)})
		}

	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("unsupported kind %q", res.Kind)})
	}
}

// ---- agents ----

func (s *Server) listAgents(c *gin.Context) {
	agents, err := s.controller.ListAgents(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if agents == nil {
		agents = []v1.AgentRecord{}
	}
	c.JSON(http.StatusOK, agents)
}

func (s *Server) getAgent(c *gin.Context) {
	agent, err := s.controller.GetAgent(c.Request.Context(), c.Param("name"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, agent)
}

func (s *Server) deleteAgent(c *gin.Context) {
	name := c.Param("name")
	if err := s.controller.DeleteAgent(c.Request.Context(), name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("agent/%s deleted", name)})
}

func (s *Server) startAgent(c *gin.Context) {
	name := c.Param("name")
	if err := s.controller.StartAgent(c.Request.Context(), name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("agent/%s started", name)})
}

func (s *Server) stopAgent(c *gin.Context) {
	name := c.Param("name")
	if err := s.controller.StopAgent(c.Request.Context(), name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("agent/%s stopped", name)})
}

func (s *Server) restartAgent(c *gin.Context) {
	name := c.Param("name")
	if err := s.controller.RestartAgent(c.Request.Context(), name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("agent/%s restarted", name)})
}

// ---- tasks ----

func (s *Server) runTask(c *gin.Context) {
	var req struct {
		Agent   string `json:"agent"`
		Message string `json:"message"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	if req.Agent == "" || req.Message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent and message are required"})
		return
	}

	ctx := c.Request.Context()
	taskID := fmt.Sprintf("task-%d", time.Now().UnixNano()/1e6)
	task := v1.TaskRecord{
		ID: taskID, AgentName: req.Agent, Message: req.Message,
		Status: v1.TaskStatusPending, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}

	if err := s.controller.Store().CreateTask(ctx, task); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	result, err := s.controller.ExecuteTask(ctx, task)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"taskId": taskID,
			"error":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"taskId":   taskID,
		"output":   result.Output,
		"tokensIn": result.TokensIn,
		"tokensOut": result.TokensOut,
	})
}

func (s *Server) listTasks(c *gin.Context) {
	tasks, err := s.controller.Store().ListTasks(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if tasks == nil {
		tasks = []v1.TaskRecord{}
	}
	c.JSON(http.StatusOK, tasks)
}

func (s *Server) getTask(c *gin.Context) {
	task, err := s.controller.Store().GetTask(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, task)
}

// ---- metrics ----

func (s *Server) agentMetrics(c *gin.Context) {
	c.JSON(http.StatusOK, s.controller.AgentMetrics())
}

func (s *Server) agentHealth(c *gin.Context) {
	c.JSON(http.StatusOK, s.controller.Health())
}

// ---- workflows ----

func (s *Server) listWorkflows(c *gin.Context) {
	wfs, err := s.controller.Store().ListWorkflows(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if wfs == nil {
		wfs = []v1.WorkflowRecord{}
	}
	c.JSON(http.StatusOK, wfs)
}

func (s *Server) deleteWorkflow(c *gin.Context) {
	name := c.Param("name")
	if err := s.controller.Store().DeleteWorkflow(c.Request.Context(), name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("workflow/%s deleted", name)})
}

func (s *Server) runWorkflow(c *gin.Context) {
	name := c.Param("name")
	ctx := c.Request.Context()

	wf, err := s.controller.Store().GetWorkflow(ctx, name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	spec, err := workflow.ParseWorkflow([]byte(wf.SpecYAML))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	engine := workflow.NewEngine(s.controller, s.controller.Store(), s.logger)
	run, err := engine.Execute(ctx, spec)
	if err != nil {
		resp := gin.H{"error": err.Error()}
		if run != nil {
			resp["run"] = run
		}
		c.JSON(http.StatusInternalServerError, resp)
		return
	}
	c.JSON(http.StatusOK, run)
}
