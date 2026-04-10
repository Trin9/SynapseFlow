# Synapse

<div align="center">
  <img src="https://img.shields.io/badge/Golang-1.23+-00ADD8?style=flat-square&logo=go" alt="Golang" />
  <img src="https://img.shields.io/badge/@xyflow/react-v12-61DAFB?style=flat-square&logo=react" alt="React Flow v12" />
  <img src="https://img.shields.io/badge/MCP-Compatible-blueviolet?style=flat-square" alt="MCP Server/Client" />
  <img src="https://img.shields.io/badge/Engine-DAG_+_Statechart-orange?style=flat-square" alt="DAG Engine" />
  <img src="https://img.shields.io/badge/DB-PostgreSQL_+_pgvector-336791?style=flat-square&logo=postgresql" alt="PostgreSQL" />
</div>

<br />

**Synapse** is a next-generation intelligent operations and automated troubleshooting engine based on Agentic Workflows. It addresses the core pain points encountered when implementing AI in SRE and DevOps environments: the high cost of pure LLM reasoning, hallucination leading to deviation from SOPs, and the lack of specific memory reuse.

The name "Synapse" originates from biology, symbolizing the platform's role as a reliable connection hub that transmits information and states between neurons (various script tools and Large Language Models).

## Core Design Philosophy

1. **Flow Engineering First**: SOPs are solidified into explicit "Hard Nodes (automated tool workflows)" and "Soft Nodes (LLM brain judgment)". This rejects the approach of allowing LLMs to run the entire process, saving 50-70% of Token consumption and reducing execution time from minutes to seconds.
2. **MCP Plug-and-Play**: The backend acts as a dynamic MCP Client hub. All enterprise log queries, database reads, and business scripts are encapsulated as MCP Servers, dynamically discovered and mounted at runtime.
3. **Dual-Loop RAG Memory**: A background "Shadow Extraction Agent" automatically extracts troubleshooting topology and root cause after each successful resolution, storing it as vectorized experience for precise recall next time.
4. **Lightweight & High-Concurrency**: Written in **Golang 1.23+**, utilizing Goroutines and Channels for natural concurrent DAG execution. Single binary deployment with PostgreSQL for persistence.

## Tech Stack

| Layer | Technology |
|-------|-----------|
| **Backend** | Golang 1.23+, Gin, PostgreSQL, pgvector, mark3labs/mcp-go |
| **Frontend** | React 18, @xyflow/react v12, TypeScript 5, Tailwind CSS, Zustand, shadcn/ui |
| **AI** | Anthropic Claude / OpenAI GPT (via structured JSON output) |
| **Observability** | Prometheus metrics, structured JSON logging (zap) |

## Documentation

| Document | Description |
|----------|------------|
| [Product Design](./docs/PRODUCT_DESIGN.md) | Product philosophy, architecture, NFRs, competitive analysis |
| [Implementation Plan](./docs/IMPLEMENTATION_PLAN.md) | Phased milestone plan with time estimates and acceptance criteria |
| [Project Structure](./docs/PROJECT_STRUCTURE.md) | Directory topology and module architecture |
| [Agent Design Principles](./AGENTS.md) | Hard/soft node collaboration, state machine flow, shadow memory |
| [Design Rationale](./docs/talk&thoughts/DESIGN_RATIONALE.md) | Architectural decision records (Q&A format) |

> *For Chinese documentation, see the `*_CN.md` files (e.g., [README_CN.md](./README_CN.md)).*

## Quick Start

*(The project is currently in the design/scaffolding phase. Implementation begins with M1 Sprint 1.)*

### Backend
```bash
cd backend
go mod tidy
make run
```

### Frontend
```bash
cd frontend
npm install
npm run dev
```

### Full Stack (Docker)
```bash
docker-compose up
```

## Project Status

| Milestone | Status | Target |
|-----------|--------|--------|
| M1: Runnable MVP | 🔲 Not Started | Core engine + canvas + hard/soft nodes |
| M2: Production-Ready | 🔲 Not Started | MCP + routing + auth + deployment |
| M3: Differentiation | 🔲 Not Started | RAG memory + advanced UI |
| M4: Experimental | 🔲 Not Started | WebMCP browser automation |

## Inspiration & Acknowledgments

Part of Synapse's architectural thought is drawn from advanced open-source engineering designs (such as `claude-code`'s dynamic MCP tool mounting and memory extraction sub-agent, and `claw-code`'s finite state machine routing philosophy).

## License

MIT License
