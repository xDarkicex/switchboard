// Package switchboard is the public entry point. The assembled gateway:
// pipeline + plug + trunk + board + engine, wired through a nanite router.
package switchboard

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/xDarkicex/nanite"
	nanitesse "github.com/xDarkicex/nanite/sse"
	"gopkg.in/yaml.v3"

	"github.com/xDarkicex/switchboard/internal/board"
	"github.com/xDarkicex/switchboard/internal/dimensions"
	"github.com/xDarkicex/switchboard/internal/lines"
	"github.com/xDarkicex/switchboard/internal/operator"
	"github.com/xDarkicex/switchboard/internal/plug"
	"github.com/xDarkicex/switchboard/internal/trunk"
)

const (
	colorReset  = "\033[0m"
	colorDim    = "\033[2m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
)

// Server is the assembled switchboard.
type Server struct {
	Patching
	pipeline *operator.Pipeline
	engine   *lines.Engine
	board    *board.Config
	router   *nanite.Router
	secrets  map[string]string
}

// Patching is the routing entry point. server.Patching.Through(w, r).
type Patching struct {
	s *Server
}

// New assembles the gateway from a board config, resolved secrets, and a
// routing engine.
func New(cfg *board.Config, secrets map[string]string, engine *lines.Engine) (*Server, error) {
	if cfg == nil {
		return nil, fmt.Errorf("switchboard: nil config")
	}
	if engine == nil {
		return nil, fmt.Errorf("switchboard: nil routing engine")
	}
	pipeCfg := operator.PipelineConfig{
		Length:     operator.PipelineSlot{ModelPath: cfg.Pipeline.Length.ModelPath, DefaultVal: cfg.Pipeline.Length.DefaultVal, Labels: cfg.Pipeline.Length.Labels},
		Complexity: operator.PipelineSlot{ModelPath: cfg.Pipeline.Complexity.ModelPath, DefaultVal: cfg.Pipeline.Complexity.DefaultVal, Labels: cfg.Pipeline.Complexity.Labels},
		Style:      operator.PipelineSlot{ModelPath: cfg.Pipeline.Style.ModelPath, DefaultVal: cfg.Pipeline.Style.DefaultVal, Labels: cfg.Pipeline.Style.Labels},
		Quality:    operator.PipelineSlot{ModelPath: cfg.Pipeline.Quality.ModelPath, DefaultVal: cfg.Pipeline.Quality.DefaultVal, Labels: cfg.Pipeline.Quality.Labels},
		Camera:     operator.PipelineSlot{ModelPath: cfg.Pipeline.Camera.ModelPath, DefaultVal: cfg.Pipeline.Camera.DefaultVal, Labels: cfg.Pipeline.Camera.Labels},
		Physics:    operator.PipelineSlot{ModelPath: cfg.Pipeline.Physics.ModelPath, DefaultVal: cfg.Pipeline.Physics.DefaultVal, Labels: cfg.Pipeline.Physics.Labels},
		Refs:       operator.PipelineSlot{ModelPath: cfg.Pipeline.Refs.ModelPath, DefaultVal: cfg.Pipeline.Refs.DefaultVal, Labels: cfg.Pipeline.Refs.Labels},
		Cost:       operator.PipelineSlot{ModelPath: cfg.Pipeline.Cost.ModelPath, DefaultVal: cfg.Pipeline.Cost.DefaultVal, Labels: cfg.Pipeline.Cost.Labels},
	}
	pipeline, err := operator.NewPipeline(pipeCfg)
	if err != nil {
		return nil, err
	}
	s := &Server{
		pipeline: pipeline,
		engine:   engine,
		board:    cfg,
		secrets:  secrets,
		router:   nanite.New(),
	}
	s.Patching = Patching{s: s}
	s.wireRoutes()
	return s, nil
}

// NewFromFiles assembles the gateway from config and routing files.
func NewFromFiles(configPath, routingPath string) (*Server, error) {
	cfg, err := board.LoadFile(configPath)
	if err != nil {
		return nil, err
	}
	engine, err := loadEngine(routingPath)
	if err != nil {
		return nil, err
	}
	return New(&cfg, cfg.ResolveSecrets(), engine)
}

func loadEngine(path string) (*lines.Engine, error) {
	if path == "" {
		return lines.NewEngine(nil), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("switchboard: read routing %q: %w", path, err)
	}
	var spec lines.EngineSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("switchboard: parse routing %q: %w", path, err)
	}
	return lines.NewEngine(spec.Rules), nil
}

func (s *Server) Close() error { return s.pipeline.Close() }

func (s *Server) GoOnDuty() error { return s.router.Start(s.board.Server.Listen) }

func (s *Server) GoOffDuty(ctx context.Context) error {
	return s.router.Shutdown(s.board.Server.WriteTimeout)
}

func (s *Server) CordTest() error {
	tags, err := s.pipeline.Classify("test")
	if err != nil {
		return err
	}
	if tags.Cost == "" {
		return fmt.Errorf("pipeline returned empty cost")
	}
	return nil
}

func (s *Server) Pipeline_Classify(text string) dimensions.Tags {
	tags, _ := s.pipeline.Classify(text)
	return tags
}

func (s *Server) Engine_Route(tags dimensions.Tags) (provider, model string) {
	return s.engine.Route(tags)
}

func (p *Patching) Through(w http.ResponseWriter, r *http.Request) {
	p.s.router.ServeHTTP(w, r)
}

func (s *Server) wireRoutes() {
	s.router.Use(s.logCall, s.classifyAndRoute())
	s.router.Post("/v1/chat/completions", s.handleChat)
	nanitesse.Register(s.router, "/v1/chat/stream", s.handleChatSSE)
	s.router.Get("/health", s.handleHealth)
	s.router.Get("/v1/ready", s.handleHealth)
}

// logCall writes each call's routing decision to stderr in operator-voice.
func (s *Server) logCall(c *nanite.Context, next func()) {
	start := time.Now()
	next()
	elapsed := time.Since(start)

	provider, _ := c.Get("switchboard.provider")
	model, _ := c.Get("switchboard.model")
	tagsI, _ := c.Get("switchboard.tags")
	tags, _ := tagsI.(dimensions.Tags)

	providerStr, _ := provider.(string)
	modelStr, _ := model.(string)
	if providerStr == "" {
		return
	}

	tieLine := fmt.Sprintf("%s/%s", providerStr, modelStr)
	style := tags.Style
	if style == "" {
		style = "—"
	}

	fmt.Fprintf(os.Stderr, "  %s◈%s %-7s %s→%s %-24s  %s%-12s%s  %s%-6s%s  %s%6s%s\n",
		colorGreen, colorReset,
		c.Request.Method,
		colorCyan, colorReset,
		truncate(c.Request.URL.Path, 24),
		colorYellow, style, colorReset,
		colorBlue, tags.Length, colorReset,
		colorDim, roundDuration(elapsed), colorReset,
	)
	_ = tieLine
}

func formatTags(t dimensions.Tags) string {
	return fmt.Sprintf("length=%s complexity=%s style=%s quality=%s camera=%s physics=%s refs=%s cost=%s",
		t.Length, t.Complexity, t.Style, t.Quality, t.Camera, t.Physics, t.Refs, t.Cost)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func roundDuration(d time.Duration) string {
	if d < time.Microsecond {
		return fmt.Sprintf("%dns", d.Nanoseconds())
	}
	if d < time.Millisecond {
		return fmt.Sprintf("%.0fµs", float64(d.Nanoseconds())/1000)
	}
	if d < time.Second {
		return fmt.Sprintf("%.1fms", float64(d.Microseconds())/1000)
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

func (s *Server) classifyAndRoute() nanite.MiddlewareFunc {
	return func(c *nanite.Context, next func()) {
		if c.Request.Body == nil {
			next()
			return
		}
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			trunk.Drop(c.Writer, c.Request, trunk.Unauthorized)
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		query := extractLastUserMessage(body)
		if query == "" {
			next()
			return
		}
		tags, err := s.pipeline.Classify(query)
		if err != nil {
			trunk.Drop(c.Writer, c.Request, trunk.CircuitOpen)
			return
		}
		providerName, modelName := s.engine.Route(tags)
		c.Set("switchboard.tags", tags)
		c.Set("switchboard.provider", providerName)
		c.Set("switchboard.model", modelName)
		c.Set("switchboard.body", body)
		// Stamp the response so callers can see the operator's decision.
		c.Writer.Header().Set("X-Operator-Provider", providerName)
		c.Writer.Header().Set("X-Operator-Model", modelName)
		c.Writer.Header().Set("X-Operator-Tags", formatTags(tags))
		next()
	}
}

func (s *Server) handleChat(c *nanite.Context) {
	providerName, _ := c.Get("switchboard.provider")
	modelName, _ := c.Get("switchboard.model")
	providerNameStr, _ := providerName.(string)
	modelNameStr, _ := modelName.(string)
	if providerNameStr == "" {
		trunk.Drop(c.Writer, c.Request, trunk.Unauthorized)
		return
	}
	tieLine := s.buildTieLine(providerNameStr)
	if tieLine == nil {
		trunk.NumberChanged(c.Writer, c.Request, providerNameStr)
		return
	}
	if err := plug.In(tieLine, c.Writer, c.Request); err != nil {
		trunk.Drop(c.Writer, c.Request, trunk.AllCircuitsDown)
	}
	_ = modelNameStr
}

// handleChatSSE handles streaming requests via nanite/sse. The nanite/sse
// package provides zero-alloc buffer management and proper flush semantics.
func (s *Server) handleChatSSE(conn *nanitesse.Connection, c *nanite.Context) {
	providerName, _ := c.Get("switchboard.provider")
	modelName, _ := c.Get("switchboard.model")
	providerNameStr, _ := providerName.(string)
	modelNameStr, _ := modelName.(string)
	if providerNameStr == "" {
		trunk.Drop(c.Writer, c.Request, trunk.Unauthorized)
		return
	}
	tieLine := s.buildTieLine(providerNameStr)
	if tieLine == nil {
		trunk.NumberChanged(c.Writer, c.Request, providerNameStr)
		return
	}
	if err := plug.StreamSSE(tieLine, c.Request, conn); err != nil {
		trunk.Drop(c.Writer, c.Request, trunk.AllCircuitsDown)
	}
	_ = modelNameStr
}

func (s *Server) buildTieLine(providerName string) *plug.TieLine {
	provider := s.board.ProviderByName(providerName)
	if provider == nil {
		return nil
	}
	return &plug.TieLine{
		Name:    provider.Name,
		BaseURL: provider.BaseURL,
		APIKey:  s.secrets[provider.Name],
		Timeout: s.board.Defaults.Timeout,
	}
}

func (s *Server) handleHealth(c *nanite.Context) {
	c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func extractLastUserMessage(body []byte) string {
	var req struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
		Prompt string `json:"prompt"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return ""
	}
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			return req.Messages[i].Content
		}
	}
	return req.Prompt
}
