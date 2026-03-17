package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	v1 "github.com/zlc-ai/opc-platform/api/v1"
	"github.com/zlc-ai/opc-platform/pkg/controller"
	"github.com/zlc-ai/opc-platform/pkg/cost"
	"github.com/zlc-ai/opc-platform/pkg/federation"
	"github.com/zlc-ai/opc-platform/pkg/gateway"
	"github.com/zlc-ai/opc-platform/pkg/goal"
	"github.com/zlc-ai/opc-platform/pkg/workflow"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// Server provides HTTP API endpoints for the OPC daemon.
type Server struct {
	controller *controller.Controller
	costMgr    *cost.Tracker
	gateway    *gateway.Gateway
	federation *federation.FederationController
	logger       *zap.SugaredLogger
	httpServer   *http.Server
	config       Config
	aiDecomposer *goal.AIDecomposer
	retryQueue   *federation.RetryQueue
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
	fedCtrl *federation.FederationController,
	config Config,
	logger *zap.SugaredLogger,
) *Server {
	if config.Port == 0 {
		config.Port = 9527
	}
	if config.Host == "" {
		config.Host = "127.0.0.1"
	}
	adapter := &controllerAdapter{ctrl: ctrl}
	return &Server{
		controller:   ctrl,
		costMgr:      costMgr,
		gateway:      gw,
		federation:   fedCtrl,
		logger:       logger,
		config:       config,
		aiDecomposer: goal.NewAIDecomposer(adapter, logger),
		retryQueue:   federation.NewRetryQueue(logger),
	}
}

// controllerAdapter bridges controller.Controller to goal.AgentController interface.
type controllerAdapter struct{ ctrl *controller.Controller }

func (a *controllerAdapter) ExecuteTask(ctx context.Context, task v1.TaskRecord) (goal.ExecuteResult, error) {
	result, err := a.ctrl.ExecuteTask(ctx, task)
	if err != nil {
		return goal.ExecuteResult{}, err
	}
	return goal.ExecuteResult{Output: result.Output, TokensIn: result.TokensIn, TokensOut: result.TokensOut}, nil
}
func (a *controllerAdapter) Apply(ctx context.Context, spec v1.AgentSpec) error {
	return a.ctrl.Apply(ctx, spec)
}
func (a *controllerAdapter) StartAgent(ctx context.Context, name string) error {
	return a.ctrl.StartAgent(ctx, name)
}
func (a *controllerAdapter) GetAgent(ctx context.Context, name string) (v1.AgentRecord, error) {
	return a.ctrl.GetAgent(ctx, name)
}

// Start starts the HTTP server.
func (s *Server) Start(ctx context.Context) error {
	// Start retry queue for failed federation callbacks.
	if s.retryQueue != nil {
		go s.retryQueue.ProcessLoop(ctx)
	}

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

		api.GET("/tasks/:id/logs", s.getTaskLogs)

		// --- metrics ---
		api.GET("/metrics", s.clusterMetrics)
		api.GET("/metrics/agents", s.agentMetrics)
		api.GET("/metrics/health", s.agentHealth)

		// --- costs ---
		api.GET("/costs/daily", s.costDaily)
		api.GET("/costs/events", s.costEvents)

		// --- logs ---
		api.GET("/logs", s.getLogs)

		// --- workflows ---
		api.GET("/workflows", s.listWorkflows)
		api.DELETE("/workflows/:name", s.deleteWorkflow)
		api.POST("/workflows/:name/run", s.runWorkflow)
		api.PUT("/workflows/:name/toggle", s.toggleWorkflow)
		api.GET("/workflows/:name/runs", s.listWorkflowRuns)
		api.GET("/workflows/:name/runs/:id", s.getWorkflowRun)

		// --- federation ---
		api.GET("/federation/companies", s.listCompanies)
		api.GET("/federation/companies/:id", s.getCompany)
		api.POST("/federation/companies", s.registerCompany)
		api.DELETE("/federation/companies/:id", s.unregisterCompany)
		api.PUT("/federation/companies/:id/status", s.updateCompanyStatus)
		api.POST("/federation/intervene", s.intervene)

		// --- federation proxy (cross-company operations) ---
		api.GET("/federation/companies/:id/agents", s.federatedAgents)
		api.GET("/federation/companies/:id/tasks", s.federatedTasks)
		api.GET("/federation/companies/:id/metrics", s.federatedMetrics)
		api.GET("/federation/companies/:id/health", s.federatedHealth)
		api.GET("/federation/aggregate/agents", s.aggregateAgents)
		api.GET("/federation/aggregate/metrics", s.aggregateMetrics)

		// --- federation callback & federated goals ---
		api.POST("/federation/callback", s.federationCallback)
		api.POST("/goals/federated", s.createFederatedGoal)

		// --- goals ---
		api.GET("/goals", s.listGoals)
		api.GET("/goals/:id", s.getGoal)
		api.POST("/goals", s.createGoal)
		api.PUT("/goals/:id", s.updateGoal)
		api.DELETE("/goals/:id", s.deleteGoal)
		api.GET("/goals/:id/projects", s.listProjectsByGoal)
		api.GET("/goals/:id/stats", s.goalStats)
		api.GET("/goals/:id/plan", s.getGoalPlan)
		api.POST("/goals/:id/approve", s.approveGoal)
		api.POST("/goals/:id/revise", s.reviseGoal)

		// --- projects ---
		api.GET("/projects", s.listProjects)
		api.GET("/projects/:id", s.getProject)
		api.POST("/projects", s.createProject)
		api.PUT("/projects/:id", s.updateProject)
		api.DELETE("/projects/:id", s.deleteProject)
		api.GET("/projects/:id/issues", s.listIssuesByProject)
		api.GET("/projects/:id/stats", s.projectStats)

		// --- issues ---
		api.GET("/issues", s.listIssues)
		api.GET("/issues/:id", s.getIssue)
		api.POST("/issues", s.createIssue)
		api.PUT("/issues/:id", s.updateIssue)
		api.DELETE("/issues/:id", s.deleteIssue)

		// --- settings ---
		api.GET("/settings", s.getSettings)
		api.PUT("/settings", s.updateSettings)
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

	case v1.KindGoal:
		// Parse Goal YAML with optional autoDecompose and decomposition.
		var raw struct {
			Metadata struct {
				Name   string            `yaml:"name"`
				Labels map[string]string `yaml:"labels"`
			} `yaml:"metadata"`
			Spec struct {
				Description     string                    `yaml:"description"`
				Owner           string                    `yaml:"owner"`
				Deadline        string                    `yaml:"deadline"`
				TargetCompanies []string                  `yaml:"targetCompanies"`
				AutoDecompose   bool                      `yaml:"autoDecompose"`
				Approval        string                    `yaml:"approval"`
				Constraints     *v1.DecomposeConstraints  `yaml:"constraints"`
				Decomposition   *struct {
					Projects []struct {
						Name        string `yaml:"name"`
						Company     string `yaml:"company"`
						Description string `yaml:"description"`
						Tasks       []struct {
							Name        string   `yaml:"name"`
							Description string   `yaml:"description"`
							AssignAgent string   `yaml:"assignAgent"`
							DependsOn   []string `yaml:"dependsOn"`
							Issues      []struct {
								Name        string `yaml:"name"`
								Description string `yaml:"description"`
							} `yaml:"issues"`
						} `yaml:"tasks"`
					} `yaml:"projects"`
				} `yaml:"decomposition"`
			} `yaml:"spec"`
		}
		if err := yaml.Unmarshal(data, &raw); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "parse Goal: " + err.Error()})
			return
		}

		goalID := uuid.New().String()
		goalRecord := v1.GoalRecord{
			ID: goalID, Name: raw.Metadata.Name, Description: raw.Spec.Description,
			Owner: raw.Spec.Owner, Deadline: raw.Spec.Deadline, Status: "active",
			SpecYAML: string(data),
		}
		if err := s.controller.Store().CreateGoal(ctx, goalRecord); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Auto-decompose: create child projects, tasks, and issues from YAML decomposition.
		var projectCount, taskCount, issueCount, dispatchedTasks int

		if raw.Spec.Decomposition != nil {
			for _, p := range raw.Spec.Decomposition.Projects {
				projectID := uuid.New().String()
				project := v1.ProjectRecord{
					ID: projectID, Name: p.Name, GoalID: goalID,
					Description: p.Description, Status: "active",
				}
				if err := s.controller.Store().CreateProject(ctx, project); err != nil {
					s.logger.Warnw("failed to create project", "name", p.Name, "error", err)
					continue
				}
				projectCount++

				for _, t := range p.Tasks {
					taskIssueID := uuid.New().String()
					taskIssue := v1.IssueRecord{
						ID: taskIssueID, Name: t.Name, ProjectID: projectID,
						Description: t.Description, AgentRef: t.AssignAgent, Status: "open",
					}
					if err := s.controller.Store().CreateIssue(ctx, taskIssue); err != nil {
						s.logger.Warnw("failed to create issue", "name", t.Name, "error", err)
						continue
					}
					taskCount++

					for _, i := range t.Issues {
						subIssue := v1.IssueRecord{
							ID: uuid.New().String(), Name: i.Name, ProjectID: projectID,
							Description: i.Description, Status: "open",
						}
						if err := s.controller.Store().CreateIssue(ctx, subIssue); err != nil {
							s.logger.Warnw("failed to create sub-issue", "name", i.Name, "error", err)
						}
						issueCount++
					}

					// Auto-dispatch: create and start agent, execute task.
					if t.AssignAgent != "" {
						if _, getErr := s.controller.GetAgent(ctx, t.AssignAgent); getErr != nil {
							autoSpec := v1.AgentSpec{
								APIVersion: v1.APIVersion, Kind: v1.KindAgentSpec,
								Metadata: v1.Metadata{Name: t.AssignAgent},
								Spec: v1.AgentSpecBody{
									Type:     v1.AgentTypeClaudeCode,
									Runtime:  v1.RuntimeConfig{Model: v1.ModelConfig{Provider: "anthropic", Name: "claude-sonnet-4"}, Timeout: v1.TimeoutConfig{Task: "600s"}},
									Context:  v1.ContextConfig{Workdir: "/tmp/opc"},
									Recovery: v1.RecoveryConfig{Enabled: true, MaxRestarts: 3},
								},
							}
							if applyErr := s.controller.Apply(ctx, autoSpec); applyErr == nil {
								s.controller.StartAgent(ctx, t.AssignAgent)
							}
						}

						taskID := fmt.Sprintf("task-%d", time.Now().UnixNano()/1e6)
						taskRecord := v1.TaskRecord{
							ID: taskID, AgentName: t.AssignAgent,
							Message:   fmt.Sprintf("[Goal: %s] [Project: %s] %s\n\n%s", raw.Metadata.Name, p.Name, t.Name, t.Description),
							Status:    v1.TaskStatusPending,
							IssueID:   taskIssueID, ProjectID: projectID, GoalID: goalID,
							CreatedAt: time.Now(), UpdatedAt: time.Now(),
						}
						if err := s.controller.Store().CreateTask(ctx, taskRecord); err == nil {
							go func(tr v1.TaskRecord) {
								bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
								defer cancel()
								s.controller.ExecuteTask(bgCtx, tr)
							}(taskRecord)
							dispatchedTasks++
						}
						time.Sleep(time.Millisecond)
					}
				}
			}
		}

		msg := fmt.Sprintf("goal/%s created", goalRecord.Name)
		if projectCount > 0 {
			msg += fmt.Sprintf(" (decomposed: %d projects, %d tasks, %d issues, %d dispatched)", projectCount, taskCount, issueCount, dispatchedTasks)
		}
		c.JSON(http.StatusOK, gin.H{"message": msg})

	case "Company":
		if s.federation == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "federation not enabled"})
			return
		}
		var reg struct {
			Metadata struct{ Name string `yaml:"name"` } `yaml:"metadata"`
			Spec struct {
				Type     string        `yaml:"type"`
				Endpoint string        `yaml:"endpoint"`
				Agents   []interface{} `yaml:"agents"`
			} `yaml:"spec"`
		}
		if err := yaml.Unmarshal(data, &reg); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "parse Company: " + err.Error()})
			return
		}
		var agentNames []string
		for _, a := range reg.Spec.Agents {
			switch v := a.(type) {
			case string:
				agentNames = append(agentNames, v)
			case map[string]interface{}:
				if name, ok := v["name"].(string); ok {
					agentNames = append(agentNames, name)
				}
			}
		}
		company, err := s.federation.RegisterCompany(federation.CompanyRegistration{
			Name: reg.Metadata.Name, Endpoint: reg.Spec.Endpoint,
			Type: federation.CompanyType(reg.Spec.Type), Agents: agentNames,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("company/%s registered (id=%s)", company.Name, company.ID)})

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
		Agent       string `json:"agent"`
		Message     string `json:"message"`
		CallbackURL string `json:"callbackURL,omitempty"`
		GoalID      string `json:"goalId,omitempty"`
		ProjectID   string `json:"projectId,omitempty"`
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

	// Execute asynchronously so the caller can poll for status.
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		result, execErr := s.controller.ExecuteTask(bgCtx, task)

		// If a callbackURL was provided, notify the originating OPC.
		if req.CallbackURL != "" {
			cb := FederationCallback{
				GoalID:    req.GoalID,
				ProjectID: req.ProjectID,
				TaskID:    taskID,
			}
			if execErr != nil {
				cb.Status = "failed"
				cb.Result = execErr.Error()
			} else {
				cb.Status = "completed"
				cb.Result = result.Output
				cb.TokensIn = result.TokensIn
				cb.TokensOut = result.TokensOut
			}
			s.sendCallback(req.CallbackURL, cb)
		}
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"taskId":  taskID,
		"status":  string(v1.TaskStatusPending),
		"message": "task accepted, poll GET /api/tasks/" + taskID + " for status",
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
	// Parse steps from specYAML so frontend gets structured data.
	type workflowWithSteps struct {
		v1.WorkflowRecord
		Steps []v1.WorkflowStep `json:"steps"`
	}
	result := make([]workflowWithSteps, 0, len(wfs))
	for _, wf := range wfs {
		wws := workflowWithSteps{WorkflowRecord: wf}
		if wf.SpecYAML != "" {
			var spec v1.WorkflowSpec
			if yaml.Unmarshal([]byte(wf.SpecYAML), &spec) == nil {
				wws.Steps = spec.Spec.Steps
			}
		}
		result = append(result, wws)
	}
	c.JSON(http.StatusOK, result)
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

// ---- cluster metrics (dashboard aggregate) ----

func (s *Server) clusterMetrics(c *gin.Context) {
	ctx := c.Request.Context()
	agents, _ := s.controller.ListAgents(ctx)
	tasks, _ := s.controller.Store().ListTasks(ctx)

	var runningAgents int
	for _, a := range agents {
		if a.Phase == v1.AgentPhaseRunning {
			runningAgents++
		}
	}

	var runningTasks, completedTasks, failedTasks int
	var todayCost float64
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	for _, t := range tasks {
		switch t.Status {
		case v1.TaskStatusRunning:
			runningTasks++
		case v1.TaskStatusCompleted:
			completedTasks++
		case v1.TaskStatusFailed:
			failedTasks++
		}
		if !t.CreatedAt.Before(startOfDay) {
			todayCost += t.Cost
		}
	}

	var monthCost, dailyBudget, monthlyBudget float64
	if s.costMgr != nil {
		status := s.costMgr.GetBudgetStatus()
		monthCost = status.MonthlySpent
		dailyBudget = status.DailyLimit
		monthlyBudget = status.MonthlyLimit
		todayCost = status.DailySpent
	}

	c.JSON(http.StatusOK, gin.H{
		"totalAgents":    len(agents),
		"runningAgents":  runningAgents,
		"totalTasks":     len(tasks),
		"runningTasks":   runningTasks,
		"completedTasks": completedTasks,
		"failedTasks":    failedTasks,
		"todayCost":      todayCost,
		"monthCost":      monthCost,
		"dailyBudget":    dailyBudget,
		"monthlyBudget":  monthlyBudget,
	})
}

// ---- costs ----

func (s *Server) costDaily(c *gin.Context) {
	if s.costMgr == nil {
		c.JSON(http.StatusOK, []gin.H{})
		return
	}

	// Aggregate costs per day for the last 7 days.
	now := time.Now()
	days := make([]gin.H, 7)
	for i := 6; i >= 0; i-- {
		day := now.AddDate(0, 0, -i)
		dateStr := day.Format("2006-01-02")
		report := s.costMgr.GenerateReport("", 24*time.Hour*time.Duration(i+1))
		days[6-i] = gin.H{"date": dateStr, "cost": report.TotalCost}
	}

	// Re-compute properly: get report per each day.
	result := make([]gin.H, 0, 7)
	for i := 6; i >= 0; i-- {
		day := now.AddDate(0, 0, -i)
		dateStr := day.Format("2006-01-02")

		// Get costs for this specific day by computing diff.
		// Simpler approach: use the cost tracker's events directly.
		startOfDay := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())
		endOfDay := startOfDay.Add(24 * time.Hour)

		dayCost := s.costMgr.DayCost(startOfDay, endOfDay)
		result = append(result, gin.H{"date": dateStr, "cost": dayCost})
	}
	c.JSON(http.StatusOK, result)
}

func (s *Server) costEvents(c *gin.Context) {
	if s.costMgr == nil {
		c.JSON(http.StatusOK, []gin.H{})
		return
	}

	events := s.costMgr.RecentEvents(100)
	c.JSON(http.StatusOK, events)
}

// ---- logs ----

func (s *Server) getLogs(c *gin.Context) {
	// Generate log entries from task records (until dedicated log storage is implemented).
	ctx := c.Request.Context()
	tasks, err := s.controller.Store().ListTasks(ctx)
	if err != nil {
		c.JSON(http.StatusOK, []gin.H{})
		return
	}
	logs := make([]gin.H, 0)
	for _, t := range tasks {
		level := "info"
		msg := fmt.Sprintf("[%s] %s → %s", t.AgentName, truncateStr(t.Message, 80), t.Status)
		if t.Status == v1.TaskStatusFailed {
			level = "error"
			if t.Error != "" {
				msg = fmt.Sprintf("[%s] FAILED: %s", t.AgentName, truncateStr(t.Error, 120))
			}
		}
		entry := gin.H{
			"timestamp": t.CreatedAt.Format(time.RFC3339),
			"level":     level,
			"message":   msg,
			"agent":     t.AgentName,
			"taskId":    t.ID,
		}
		if t.GoalID != "" {
			entry["fields"] = gin.H{"goalId": t.GoalID, "projectId": t.ProjectID}
		}
		logs = append(logs, entry)
	}
	// Reverse to show newest first.
	for i, j := 0, len(logs)-1; i < j; i, j = i+1, j-1 {
		logs[i], logs[j] = logs[j], logs[i]
	}
	c.JSON(http.StatusOK, logs)
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen { return s }
	return s[:maxLen] + "..."
}

// ---- federation ----

func (s *Server) listCompanies(c *gin.Context) {
	if s.federation == nil {
		c.JSON(http.StatusOK, []interface{}{})
		return
	}
	companies := s.federation.ListCompanies()
	if companies == nil {
		companies = []*federation.Company{}
	}
	c.JSON(http.StatusOK, companies)
}

func (s *Server) getCompany(c *gin.Context) {
	if s.federation == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "federation not initialized"})
		return
	}
	company, err := s.federation.GetCompany(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, company)
}

func (s *Server) registerCompany(c *gin.Context) {
	if s.federation == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "federation not initialized"})
		return
	}
	var reg federation.CompanyRegistration
	if err := c.ShouldBindJSON(&reg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	if reg.Name == "" || reg.Endpoint == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name and endpoint are required"})
		return
	}
	company, err := s.federation.RegisterCompany(reg)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, company)
}

func (s *Server) unregisterCompany(c *gin.Context) {
	if s.federation == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "federation not initialized"})
		return
	}
	id := c.Param("id")
	if err := s.federation.UnregisterCompany(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("company/%s unregistered", id)})
}

func (s *Server) updateCompanyStatus(c *gin.Context) {
	if s.federation == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "federation not initialized"})
		return
	}
	id := c.Param("id")
	var req struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	if err := s.federation.UpdateCompanyStatus(id, federation.CompanyStatus(req.Status)); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("company/%s status updated to %s", id, req.Status)})
}

func (s *Server) intervene(c *gin.Context) {
	if s.federation == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "federation not initialized"})
		return
	}
	var req struct {
		IssueID string `json:"issueId"`
		Action  string `json:"action"`
		Reason  string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	if err := s.federation.Intervene(req.IssueID, req.Action, req.Reason, "api"); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "intervention recorded"})
}

// ---- federation proxy (cross-company operations) ----

func (s *Server) resolveCompany(c *gin.Context) (*federation.Company, bool) {
	if s.federation == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "federation not initialized"})
		return nil, false
	}
	company, err := s.federation.GetCompany(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return nil, false
	}
	return company, true
}

func (s *Server) federatedAgents(c *gin.Context) {
	company, ok := s.resolveCompany(c)
	if !ok {
		return
	}
	data, err := s.federation.Transport().Send(company.Endpoint, http.MethodGet, "/api/agents", nil)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "remote company unreachable: " + err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", data)
}

func (s *Server) federatedTasks(c *gin.Context) {
	company, ok := s.resolveCompany(c)
	if !ok {
		return
	}
	data, err := s.federation.Transport().Send(company.Endpoint, http.MethodGet, "/api/tasks", nil)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "remote company unreachable: " + err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", data)
}

func (s *Server) federatedMetrics(c *gin.Context) {
	company, ok := s.resolveCompany(c)
	if !ok {
		return
	}
	data, err := s.federation.Transport().Send(company.Endpoint, http.MethodGet, "/api/metrics", nil)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "remote company unreachable: " + err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", data)
}

func (s *Server) federatedHealth(c *gin.Context) {
	company, ok := s.resolveCompany(c)
	if !ok {
		return
	}
	err := s.federation.Transport().Ping(company.Endpoint)
	healthy := err == nil
	c.JSON(http.StatusOK, gin.H{"healthy": healthy, "endpoint": company.Endpoint})
}

// federatedAgent is a JSON-serializable agent record with a company tag.
type federatedAgent struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Phase     string `json:"phase"`
	Company   string `json:"company"`
	CompanyID string `json:"companyId"`
}

func (s *Server) aggregateAgents(c *gin.Context) {
	if s.federation == nil {
		c.JSON(http.StatusOK, []interface{}{})
		return
	}

	companies := s.federation.ListCompanies()
	type result struct {
		agents []federatedAgent
		err    error
	}

	var wg sync.WaitGroup
	results := make([]result, len(companies))

	for i, comp := range companies {
		wg.Add(1)
		go func(idx int, co *federation.Company) {
			defer wg.Done()
			data, err := s.federation.Transport().Send(co.Endpoint, http.MethodGet, "/api/agents", nil)
			if err != nil {
				results[idx] = result{err: err}
				return
			}
			var raw []json.RawMessage
			if err := json.Unmarshal(data, &raw); err != nil {
				results[idx] = result{err: err}
				return
			}
			agents := make([]federatedAgent, 0, len(raw))
			for _, r := range raw {
				var a struct {
					Name  string `json:"name"`
					Type  string `json:"type"`
					Phase string `json:"phase"`
				}
				if err := json.Unmarshal(r, &a); err != nil {
					continue
				}
				agents = append(agents, federatedAgent{
					Name:      a.Name,
					Type:      a.Type,
					Phase:     a.Phase,
					Company:   co.Name,
					CompanyID: co.ID,
				})
			}
			results[idx] = result{agents: agents}
		}(i, comp)
	}

	wg.Wait()

	allAgents := make([]federatedAgent, 0)
	for _, r := range results {
		if r.err == nil {
			allAgents = append(allAgents, r.agents...)
		}
	}

	c.JSON(http.StatusOK, allAgents)
}

// aggregatedMetrics is the merged metrics across all federation companies.
type aggregatedMetrics struct {
	TotalAgents    int     `json:"totalAgents"`
	RunningAgents  int     `json:"runningAgents"`
	TotalTasks     int     `json:"totalTasks"`
	RunningTasks   int     `json:"runningTasks"`
	CompletedTasks int     `json:"completedTasks"`
	FailedTasks    int     `json:"failedTasks"`
	TodayCost      float64 `json:"todayCost"`
	MonthCost      float64 `json:"monthCost"`
	CompanyCount   int     `json:"companyCount"`
	OnlineCount    int     `json:"onlineCount"`
}

func (s *Server) aggregateMetrics(c *gin.Context) {
	if s.federation == nil {
		c.JSON(http.StatusOK, aggregatedMetrics{})
		return
	}

	companies := s.federation.ListCompanies()

	type metricsResult struct {
		data aggregatedMetrics
		ok   bool
	}

	var wg sync.WaitGroup
	results := make([]metricsResult, len(companies))

	for i, comp := range companies {
		wg.Add(1)
		go func(idx int, co *federation.Company) {
			defer wg.Done()
			data, err := s.federation.Transport().Send(co.Endpoint, http.MethodGet, "/api/metrics", nil)
			if err != nil {
				return
			}
			var m aggregatedMetrics
			if err := json.Unmarshal(data, &m); err != nil {
				return
			}
			m.CompanyCount = 1
			m.OnlineCount = 1
			results[idx] = metricsResult{data: m, ok: true}
		}(i, comp)
	}

	wg.Wait()

	agg := aggregatedMetrics{CompanyCount: len(companies)}
	for _, r := range results {
		if r.ok {
			agg.TotalAgents += r.data.TotalAgents
			agg.RunningAgents += r.data.RunningAgents
			agg.TotalTasks += r.data.TotalTasks
			agg.RunningTasks += r.data.RunningTasks
			agg.CompletedTasks += r.data.CompletedTasks
			agg.FailedTasks += r.data.FailedTasks
			agg.TodayCost += r.data.TodayCost
			agg.MonthCost += r.data.MonthCost
			agg.OnlineCount += r.data.OnlineCount
		}
	}

	c.JSON(http.StatusOK, agg)
}

// ---- federation callback ----

// FederationCallback is the payload sent by a remote OPC when a task completes.
type FederationCallback struct {
	GoalID    string  `json:"goalId"`
	ProjectID string  `json:"projectId"`
	TaskID    string  `json:"taskId"`
	Status    string  `json:"status"` // "completed" | "failed" | "milestone"
	Result    string  `json:"result,omitempty"`
	TokensIn  int     `json:"tokensIn,omitempty"`
	TokensOut int     `json:"tokensOut,omitempty"`
	Cost      float64 `json:"cost,omitempty"`
}

func (s *Server) federationCallback(c *gin.Context) {
	var cb FederationCallback
	if err := c.ShouldBindJSON(&cb); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid callback: " + err.Error()})
		return
	}

	if cb.TaskID == "" || cb.Status == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "taskId and status are required"})
		return
	}

	s.logger.Infow("federation callback received",
		"goalId", cb.GoalID,
		"projectId", cb.ProjectID,
		"taskId", cb.TaskID,
		"status", cb.Status,
		"tokensIn", cb.TokensIn,
		"tokensOut", cb.TokensOut,
		"cost", cb.Cost,
	)

	switch cb.Status {
	case "milestone":
		s.logger.Infow("milestone reached",
			"goalId", cb.GoalID,
			"projectId", cb.ProjectID,
			"taskId", cb.TaskID,
			"result", cb.Result,
		)
	case "completed", "failed":
		s.logger.Infow("remote task finished",
			"goalId", cb.GoalID,
			"projectId", cb.ProjectID,
			"taskId", cb.TaskID,
			"status", cb.Status,
		)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("callback processed for task %s", cb.TaskID),
	})
}

// sendCallback posts a FederationCallback to a remote URL.
func (s *Server) sendCallback(callbackURL string, cb FederationCallback) {
	data, err := json.Marshal(cb)
	if err != nil {
		s.logger.Warnw("failed to marshal callback", "error", err)
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(callbackURL, "application/json", bytes.NewReader(data))
	if err != nil {
		s.logger.Warnw("failed to send callback",
			"callbackURL", callbackURL,
			"taskId", cb.TaskID,
			"error", err,
		)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		s.logger.Warnw("callback returned error",
			"callbackURL", callbackURL,
			"status", resp.StatusCode,
			"body", string(body),
		)
	} else {
		s.logger.Infow("callback sent successfully",
			"callbackURL", callbackURL,
			"taskId", cb.TaskID,
		)
	}
}

// ---- federated goals ----

func (s *Server) createFederatedGoal(c *gin.Context) {
	if s.federation == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "federation not initialized"})
		return
	}

	var req struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Companies   []string `json:"companies"` // company IDs
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	if req.Name == "" || len(req.Companies) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name and at least one company are required"})
		return
	}

	goalID := fmt.Sprintf("goal-%d", time.Now().UnixNano()/1e6)
	callbackURL := fmt.Sprintf("http://%s:%d/api/federation/callback", s.config.Host, s.config.Port)

	type dispatchResult struct {
		CompanyID   string `json:"companyId"`
		CompanyName string `json:"companyName"`
		Status      string `json:"status"`
		Error       string `json:"error,omitempty"`
	}

	results := make([]dispatchResult, 0, len(req.Companies))

	for _, companyID := range req.Companies {
		company, err := s.federation.GetCompany(companyID)
		if err != nil {
			results = append(results, dispatchResult{
				CompanyID: companyID,
				Status:    "error",
				Error:     err.Error(),
			})
			continue
		}

		if company.Status != federation.CompanyStatusOnline {
			results = append(results, dispatchResult{
				CompanyID:   company.ID,
				CompanyName: company.Name,
				Status:      "skipped",
				Error:       fmt.Sprintf("company is %s", company.Status),
			})
			continue
		}

		// Pick the first agent or use a default.
		agent := "default"
		if len(company.Agents) > 0 {
			agent = company.Agents[0]
		}

		payload := map[string]interface{}{
			"agent":       agent,
			"message":     fmt.Sprintf("[Federated Goal: %s] %s", req.Name, req.Description),
			"callbackURL": callbackURL,
			"goalId":      goalID,
			"projectId":   fmt.Sprintf("proj-%s-%s", goalID, companyID),
		}

		transport := s.federation.Transport()
		_, sendErr := transport.Send(company.Endpoint, "POST", "/api/run", payload)
		if sendErr != nil {
			results = append(results, dispatchResult{
				CompanyID:   company.ID,
				CompanyName: company.Name,
				Status:      "error",
				Error:       sendErr.Error(),
			})
		} else {
			results = append(results, dispatchResult{
				CompanyID:   company.ID,
				CompanyName: company.Name,
				Status:      "dispatched",
			})
		}
	}

	c.JSON(http.StatusAccepted, gin.H{
		"goalId":      goalID,
		"name":        req.Name,
		"description": req.Description,
		"dispatched":  results,
		"callbackURL": callbackURL,
	})
}

// ---- goals CRUD ----

func (s *Server) listGoals(c *gin.Context) {
	goals, err := s.controller.Store().ListGoals(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if goals == nil { goals = []v1.GoalRecord{} }
	c.JSON(http.StatusOK, goals)
}

func (s *Server) getGoal(c *gin.Context) {
	goal, err := s.controller.Store().GetGoal(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, goal)
}

func (s *Server) createGoal(c *gin.Context) {
	var req struct {
		v1.GoalRecord
		AutoDecompose bool                     `json:"autoDecompose"`
		Approval      string                   `json:"approval"`
		Constraints   *v1.DecomposeConstraints `json:"constraints"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	g := req.GoalRecord
	if g.ID == "" { g.ID = uuid.New().String() }
	if g.Status == "" { g.Status = "active" }

	ctx := c.Request.Context()

	if req.AutoDecompose && s.aiDecomposer != nil {
		g.Phase = v1.GoalPhaseDecomposing
		g.Status = "decomposing"
		if err := s.controller.Store().CreateGoal(ctx, g); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Async AI decomposition.
		go func(goalRec v1.GoalRecord, approval string, constraints *v1.DecomposeConstraints) {
			bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()

			if constraints != nil {
				s.aiDecomposer.SetConstraints(constraints)
			}

			decompReq := goal.DecomposeRequest{
				GoalID:      goalRec.ID,
				GoalName:    goalRec.Name,
				Description: goalRec.Description,
			}
			result, err := s.aiDecomposer.Decompose(bgCtx, decompReq)
			if err != nil {
				s.logger.Errorw("AI decomposition failed", "goalId", goalRec.ID, "error", err)
				goalRec.Phase = v1.GoalPhaseFailed
				goalRec.Status = "failed"
				s.controller.Store().UpdateGoal(bgCtx, goalRec)
				return
			}

			// Persist plan.
			planJSON, _ := json.Marshal(result)
			goalRec.DecompositionPlan = string(planJSON)

			if approval == "auto" {
				goalRec.Phase = v1.GoalPhaseInProgress
				goalRec.Status = "in_progress"
				s.controller.Store().UpdateGoal(bgCtx, goalRec)
				s.logger.Infow("goal auto-approved", "goalId", goalRec.ID, "projects", len(result.Projects))
			} else {
				goalRec.Phase = v1.GoalPhasePlanned
				goalRec.Status = "planned"
				s.controller.Store().UpdateGoal(bgCtx, goalRec)
				s.logger.Infow("goal planned, awaiting approval", "goalId", goalRec.ID, "projects", len(result.Projects))
			}
		}(g, req.Approval, req.Constraints)

		c.JSON(http.StatusAccepted, gin.H{
			"id": g.ID, "name": g.Name, "phase": "decomposing",
			"message": "goal created, AI decomposition in progress",
		})
		return
	}

	// Non-autoDecompose: simple create.
	if err := s.controller.Store().CreateGoal(ctx, g); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, g)
}

func (s *Server) updateGoal(c *gin.Context) {
	var goal v1.GoalRecord
	if err := c.ShouldBindJSON(&goal); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	goal.ID = c.Param("id")
	if err := s.controller.Store().UpdateGoal(c.Request.Context(), goal); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, goal)
}

func (s *Server) deleteGoal(c *gin.Context) {
	if err := s.controller.Store().DeleteGoal(c.Request.Context(), c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "goal deleted"})
}

func (s *Server) listProjectsByGoal(c *gin.Context) {
	projects, err := s.controller.Store().ListProjectsByGoal(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if projects == nil { projects = []v1.ProjectRecord{} }
	c.JSON(http.StatusOK, projects)
}

func (s *Server) goalStats(c *gin.Context) {
	stats, err := s.controller.Store().GoalStats(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

func (s *Server) getGoalPlan(c *gin.Context) {
	goal, err := s.controller.Store().GetGoal(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if goal.DecompositionPlan == "" {
		c.JSON(http.StatusOK, gin.H{"goalId": goal.ID, "phase": goal.Phase, "plan": nil})
		return
	}
	var plan json.RawMessage
	json.Unmarshal([]byte(goal.DecompositionPlan), &plan)
	c.JSON(http.StatusOK, gin.H{"goalId": goal.ID, "phase": goal.Phase, "plan": plan})
}

func (s *Server) approveGoal(c *gin.Context) {
	goal, err := s.controller.Store().GetGoal(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if goal.Phase != v1.GoalPhasePlanned {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("goal is in phase %q, must be 'planned' to approve", goal.Phase)})
		return
	}
	goal.Phase = v1.GoalPhaseApproved
	goal.Status = "approved"
	s.controller.Store().UpdateGoal(c.Request.Context(), goal)
	c.JSON(http.StatusOK, gin.H{"message": "goal approved", "goalId": goal.ID})
}

func (s *Server) reviseGoal(c *gin.Context) {
	goal, err := s.controller.Store().GetGoal(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	var body struct { Plan json.RawMessage `json:"plan"` }
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	goal.DecompositionPlan = string(body.Plan)
	goal.Phase = v1.GoalPhasePlanned
	s.controller.Store().UpdateGoal(c.Request.Context(), goal)
	c.JSON(http.StatusOK, gin.H{"message": "goal plan revised", "goalId": goal.ID})
}

// ---- projects CRUD ----

func (s *Server) listProjects(c *gin.Context) {
	projects, err := s.controller.Store().ListProjects(c.Request.Context())
	if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()}); return }
	if projects == nil { projects = []v1.ProjectRecord{} }
	c.JSON(http.StatusOK, projects)
}

func (s *Server) getProject(c *gin.Context) {
	p, err := s.controller.Store().GetProject(c.Request.Context(), c.Param("id"))
	if err != nil { c.JSON(http.StatusNotFound, gin.H{"error": err.Error()}); return }
	c.JSON(http.StatusOK, p)
}

func (s *Server) createProject(c *gin.Context) {
	var p v1.ProjectRecord
	if err := c.ShouldBindJSON(&p); err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}); return }
	if p.ID == "" { p.ID = uuid.New().String() }
	if p.Status == "" { p.Status = "active" }
	if err := s.controller.Store().CreateProject(c.Request.Context(), p); err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()}); return }
	c.JSON(http.StatusCreated, p)
}

func (s *Server) updateProject(c *gin.Context) {
	var p v1.ProjectRecord
	if err := c.ShouldBindJSON(&p); err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}); return }
	p.ID = c.Param("id")
	if err := s.controller.Store().UpdateProject(c.Request.Context(), p); err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()}); return }
	c.JSON(http.StatusOK, p)
}

func (s *Server) deleteProject(c *gin.Context) {
	if err := s.controller.Store().DeleteProject(c.Request.Context(), c.Param("id")); err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()}); return }
	c.JSON(http.StatusOK, gin.H{"message": "project deleted"})
}

func (s *Server) listIssuesByProject(c *gin.Context) {
	issues, err := s.controller.Store().ListIssuesByProject(c.Request.Context(), c.Param("id"))
	if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()}); return }
	if issues == nil { issues = []v1.IssueRecord{} }
	c.JSON(http.StatusOK, issues)
}

func (s *Server) projectStats(c *gin.Context) {
	stats, err := s.controller.Store().ProjectStats(c.Request.Context(), c.Param("id"))
	if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()}); return }
	c.JSON(http.StatusOK, stats)
}

// ---- issues CRUD ----

func (s *Server) listIssues(c *gin.Context) {
	issues, err := s.controller.Store().ListIssues(c.Request.Context())
	if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()}); return }
	if issues == nil { issues = []v1.IssueRecord{} }
	c.JSON(http.StatusOK, issues)
}

func (s *Server) getIssue(c *gin.Context) {
	issue, err := s.controller.Store().GetIssue(c.Request.Context(), c.Param("id"))
	if err != nil { c.JSON(http.StatusNotFound, gin.H{"error": err.Error()}); return }
	c.JSON(http.StatusOK, issue)
}

func (s *Server) createIssue(c *gin.Context) {
	var issue v1.IssueRecord
	if err := c.ShouldBindJSON(&issue); err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}); return }
	if issue.ID == "" { issue.ID = uuid.New().String() }
	if issue.Status == "" { issue.Status = "open" }
	if err := s.controller.Store().CreateIssue(c.Request.Context(), issue); err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()}); return }
	c.JSON(http.StatusCreated, issue)
}

func (s *Server) updateIssue(c *gin.Context) {
	var issue v1.IssueRecord
	if err := c.ShouldBindJSON(&issue); err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}); return }
	issue.ID = c.Param("id")
	if err := s.controller.Store().UpdateIssue(c.Request.Context(), issue); err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()}); return }
	c.JSON(http.StatusOK, issue)
}

func (s *Server) deleteIssue(c *gin.Context) {
	if err := s.controller.Store().DeleteIssue(c.Request.Context(), c.Param("id")); err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()}); return }
	c.JSON(http.StatusOK, gin.H{"message": "issue deleted"})
}

// ---- task logs ----

func (s *Server) getTaskLogs(c *gin.Context) {
	task, err := s.controller.Store().GetTask(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"taskId": task.ID,
		"logs":   task.Result,
		"error":  task.Error,
		"status": task.Status,
	})
}

// ---- workflow toggle + runs ----

func (s *Server) toggleWorkflow(c *gin.Context) {
	name := c.Param("name")
	ctx := c.Request.Context()
	wf, err := s.controller.Store().GetWorkflow(ctx, name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	var body struct{ Enabled *bool `json:"enabled"` }
	if err := c.ShouldBindJSON(&body); err != nil || body.Enabled == nil {
		wf.Enabled = !wf.Enabled
	} else {
		wf.Enabled = *body.Enabled
	}
	if err := s.controller.Store().UpdateWorkflow(ctx, wf); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"name": wf.Name, "enabled": wf.Enabled})
}

func (s *Server) listWorkflowRuns(c *gin.Context) {
	name := c.Param("name")
	runs, err := s.controller.Store().ListWorkflowRuns(c.Request.Context(), name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if runs == nil { runs = []v1.WorkflowRunRecord{} }
	c.JSON(http.StatusOK, runs)
}

func (s *Server) getWorkflowRun(c *gin.Context) {
	run, err := s.controller.Store().GetWorkflowRun(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, run)
}

// ---- settings ----

func settingsDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".opc")
}

func (s *Server) getSettings(c *gin.Context) {
	data, err := os.ReadFile(filepath.Join(settingsDir(), "settings.json"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{})
		return
	}
	var settings map[string]interface{}
	json.Unmarshal(data, &settings)
	c.JSON(http.StatusOK, settings)
}

func (s *Server) updateSettings(c *gin.Context) {
	var settings map[string]interface{}
	if err := c.ShouldBindJSON(&settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	dir := settingsDir()
	os.MkdirAll(dir, 0700)
	data, _ := json.MarshalIndent(settings, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), data, 0600); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "settings saved"})
}
