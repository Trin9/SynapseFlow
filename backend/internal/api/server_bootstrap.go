package api

import (
	"context"
	"os"

	appDAG "github.com/Trin9/SynapseFlow/backend/internal/application/dag"
	appExecution "github.com/Trin9/SynapseFlow/backend/internal/application/execution"
	appOps "github.com/Trin9/SynapseFlow/backend/internal/application/ops"
	appSystem "github.com/Trin9/SynapseFlow/backend/internal/application/system"
	appWorkspace "github.com/Trin9/SynapseFlow/backend/internal/application/workspace"
	"github.com/Trin9/SynapseFlow/backend/internal/config"
	"github.com/Trin9/SynapseFlow/backend/internal/engine"
	"github.com/Trin9/SynapseFlow/backend/internal/llm"
	"github.com/Trin9/SynapseFlow/backend/internal/mcp"
	"github.com/Trin9/SynapseFlow/backend/internal/memory"
	"github.com/Trin9/SynapseFlow/backend/internal/metrics"
	"github.com/Trin9/SynapseFlow/backend/internal/notify"
	"github.com/Trin9/SynapseFlow/backend/internal/store"
	"github.com/Trin9/SynapseFlow/backend/pkg/logger"
	"github.com/Trin9/SynapseFlow/backend/pkg/models"
)

// NewServer creates and configures the HTTP server with all routes.
func NewServer(opts ...ServerOption) *Server {
	log := logger.L()
	cfg := config.Load()

	s := &Server{
		config:           cfg,
		metricsCollector: metrics.NewCollector(),
		apiKeys:          parseAPIKeys(os.Getenv("SYNAPSE_API_KEYS")),
		jwtSecret:        []byte(os.Getenv("SYNAPSE_JWT_SECRET")),
	}

	if cfg.EnableDBStorage {
		db, err := store.OpenPostgres(context.Background(), cfg.DatabaseURL, cfg.DBMaxOpenConns, cfg.DBMaxIdleConns, cfg.DBConnMaxIdle, cfg.DBConnMaxLife)
		if err != nil {
			log.Warnw("database unavailable, falling back to in-memory stores", "error", err)
		} else {
			if err := store.RunMigrations(context.Background(), db, cfg.MigrationsPath); err != nil {
				log.Warnw("database migrations failed, falling back to in-memory stores", "error", err)
				_ = db.Close()
			} else {
				stores := store.NewPostgresStores(db)
				s.db = db
				s.dags = stores.DAGs
				s.execs = stores.Executions
				s.audits = stores.Audits
				s.episodes = stores.Episodes
				s.memory = stores.MemoryStore()
			}
		}
	}

	if s.dags == nil {
		s.dags = store.NewMemoryDAGStore()
	}
	if s.execs == nil {
		s.execs = store.NewMemoryExecutionStore()
	}
	if s.audits == nil {
		s.audits = store.NewMemoryAuditStore()
	}
	if s.episodes == nil {
		s.episodes = store.NewMemoryEpisodeStore()
	}
	if s.memory == nil {
		s.memory = memory.NewInMemoryStore()
	}

	// Notification: Slack webhook URL from env (may be overridden per-DAG)
	if slackURL := os.Getenv("SYNAPSE_SLACK_WEBHOOK_URL"); slackURL != "" {
		s.notifier = &notify.SlackSender{}
		s.slackWebhookURL = slackURL
	}

	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}
	if s.mcpMgr == nil {
		s.mcpMgr = mcp.NewManager(mcp.DefaultServersConfigPath())
	}

	// Configure executors based on environment
	episodeWriter := engine.NewEpisodeWriter(s.episodes)
	s.episodeWriter = episodeWriter
	executors := map[models.NodeType]engine.NodeExecutor{
		models.NodeTypeScript: &engine.ScriptExecutor{Writer: episodeWriter},
		models.NodeTypeHuman:  &engine.HumanExecutor{Writer: episodeWriter},
		models.NodeTypeRouter: &engine.RouterExecutor{},
		models.NodeTypeMCP:    &engine.MCPExecutor{MCP: s.mcpMgr, Writer: episodeWriter},
		models.NodeTypeWebInteraction: &engine.WebNodeExecutor{
			Writer: episodeWriter,
		},
	}

	// Use real LLM executor if API key is set, otherwise mock
	apiKey := os.Getenv("LLM_API_KEY")
	if apiKey != "" {
		// Build LLM client chain: JSON enforcing -> fallback across providers
		var clients []llm.LLMClient

		// Primary provider: check LLM_PROVIDER env (default "openai")
		provider := os.Getenv("LLM_PROVIDER")
		if provider == "" {
			provider = "openai"
		}

		switch provider {
		case "anthropic":
			apiURL := os.Getenv("LLM_API_URL")
			model := os.Getenv("LLM_MODEL")
			if model == "" {
				model = "claude-sonnet-4-20250514"
			}
			clients = append(clients, llm.NewAnthropicClient(llm.ProviderConfig{
				APIURL: apiURL,
				APIKey: apiKey,
				Model:  model,
			}))
			log.Infow("LLM primary: Anthropic", "model", model)
		default: // "openai" or any OpenAI-compatible
			apiURL := os.Getenv("LLM_API_URL")
			if apiURL == "" {
				apiURL = "https://api.openai.com/v1/chat/completions"
			}
			model := os.Getenv("LLM_MODEL")
			if model == "" {
				model = "gpt-4o-mini"
			}
			clients = append(clients, llm.NewOpenAIClient(llm.ProviderConfig{
				APIURL: apiURL,
				APIKey: apiKey,
				Model:  model,
			}))
			log.Infow("LLM primary: OpenAI-compatible", "model", model, "api_url", apiURL)
		}

		// Optional fallback provider
		fallbackKey := os.Getenv("LLM_FALLBACK_API_KEY")
		if fallbackKey != "" {
			fallbackProvider := os.Getenv("LLM_FALLBACK_PROVIDER")
			fallbackURL := os.Getenv("LLM_FALLBACK_API_URL")
			fallbackModel := os.Getenv("LLM_FALLBACK_MODEL")

			switch fallbackProvider {
			case "anthropic":
				clients = append(clients, llm.NewAnthropicClient(llm.ProviderConfig{
					APIURL: fallbackURL,
					APIKey: fallbackKey,
					Model:  fallbackModel,
				}))
				log.Infow("LLM fallback: Anthropic", "model", fallbackModel)
			default:
				if fallbackURL == "" {
					fallbackURL = "https://api.openai.com/v1/chat/completions"
				}
				clients = append(clients, llm.NewOpenAIClient(llm.ProviderConfig{
					APIURL: fallbackURL,
					APIKey: fallbackKey,
					Model:  fallbackModel,
				}))
				log.Infow("LLM fallback: OpenAI-compatible", "model", fallbackModel)
			}
		}

		// Wrap in fallback (if multiple) then JSON enforcement
		var client llm.LLMClient
		if len(clients) > 1 {
			client = llm.NewFallbackClient(clients...)
		} else {
			client = clients[0]
		}
		client = llm.NewJSONEnforcingClient(client)

		executors[models.NodeTypeLLM] = &engine.LLMExecutor{Client: client, Writer: episodeWriter}
	} else {
		executors[models.NodeTypeLLM] = &engine.MockLLMExecutor{Writer: episodeWriter}
		log.Infow("Using mock LLM executor (set LLM_API_KEY for real LLM)")
	}

	s.extractor = &memory.Extractor{Store: s.memory}
	retriever := &memory.Retriever{Store: s.memory}
	s.scheduler = engine.NewScheduler(executors, retriever)
	s.scheduler.SetEpisodeStore(s.episodes)
	s.execService = &appExecution.Service{
		Scheduler:        s.scheduler,
		DAGs:             s.dags,
		Executions:       s.execs,
		Episodes:         s.episodes,
		EpisodeWriter:    s.episodeWriter,
		MetricsCollector: s.metricsCollector,
		Notifier:         s.notifier,
		GetNotifier: func() notify.Sender {
			return s.notifier
		},
		Extractor:                  s.extractor,
		ResolveSlackURL:            s.resolveSlackURL,
		BuildExecutionNotification: buildExecutionNotification,
	}
	s.dagService = &appDAG.Service{DAGs: s.dags}
	s.opsService = &appOps.Service{Audits: s.audits, Memory: s.memory}
	s.systemSvc = &appSystem.Service{MCP: s.mcpMgr}
	if s.db != nil {
		s.systemSvc.DB = s.db
	}
	s.workspaceSvc = &appWorkspace.Service{
		Executions:              s.execs,
		Episodes:                s.episodes,
		MemoryStore:             s.memory,
		EpisodeWriter:           s.episodeWriter,
		BuildTriggerContextView: buildTriggerContextView,
		BuildReplaySliceView:    buildReplaySliceView,
		BuildComparisonSummary:  buildComparisonSummary,
		BuildEpisodeDossier:     buildEpisodeDossier,
		BuildMemoryRecalls:      buildMemoryRecallsForEpisode,
		LogMemoryRecallWarning: func(episodeID string, err error) {
			logger.L().Warnw("memory recall search failed; dossier served with empty recalls",
				"episode_id", episodeID, "error", err)
		},
	}

	s.setupRouter()
	return s
}
