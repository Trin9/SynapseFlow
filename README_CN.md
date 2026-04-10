# Synapse

<div align="center">
  <img src="https://img.shields.io/badge/Golang-1.23+-00ADD8?style=flat-square&logo=go" alt="Golang" />
  <img src="https://img.shields.io/badge/@xyflow/react-v12-61DAFB?style=flat-square&logo=react" alt="React Flow v12" />
  <img src="https://img.shields.io/badge/MCP-Compatible-blueviolet?style=flat-square" alt="MCP Server/Client" />
  <img src="https://img.shields.io/badge/Engine-DAG_+_Statechart-orange?style=flat-square" alt="DAG Engine" />
  <img src="https://img.shields.io/badge/DB-PostgreSQL_+_pgvector-336791?style=flat-square&logo=postgresql" alt="PostgreSQL" />
</div>

<br />

**Synapse** 是一款基于 Agentic Workflow（智能体工作流）的下一代智能运维与自动化排查引擎。它致力于解决当前 SRE 与 DevOps 领域在落地 AI 时遇到的核心痛点：高成本的纯 LLM 推理浪费、易产生幻觉偏离主线 SOP、缺乏特定记忆复用。

Synapse 的名字来源于生物学中的"突触"，寓意着平台作为神经元（各种脚本工具与大模型）之间传递信息与状态的可靠连接枢纽。

## 核心设计理念

1. **Flow Engineering 优先**：将 SOP 固化为明确的"硬节点（自动化工具流）"和"软节点（LLM 研判）"。拒绝让大模型全程"无脑盲人摸象"，节约 50-70% 的 Token 消耗，并将执行时间从分钟级降至秒级。
2. **MCP 即插即用**：后端作为动态 MCP Client 中枢。企业内所有日志查询、数据库读取、业务脚本均封装为 MCP Server，运行时动态发现并挂载。
3. **双循环 RAG 记忆**：后台"影子提取智能体"在每次成功排查后自动提取排查拓扑和根因，存储为向量化经验，下次精准预加载。
4. **轻量与高并发**：采用 **Golang 1.23+** 编写底层引擎，利用 Goroutine/Channel 天然支持 DAG 并发执行。单二进制部署，PostgreSQL 持久化。

## 技术栈

| 层级 | 技术选型 |
|------|----------|
| **后端** | Golang 1.23+、Gin、PostgreSQL、pgvector、mark3labs/mcp-go |
| **前端** | React 18、@xyflow/react v12、TypeScript 5、Tailwind CSS、Zustand、shadcn/ui |
| **AI** | Anthropic Claude / OpenAI GPT（结构化 JSON 输出） |
| **可观测性** | Prometheus metrics、结构化 JSON 日志（zap） |

## 文档导航

| 文档 | 描述 |
|------|------|
| [产品设计规格书](./docs/PRODUCT_DESIGN_CN.md) | 产品理念、架构、非功能性需求、竞品分析 |
| [实施计划](./docs/IMPLEMENTATION_PLAN_CN.md) | 分阶段里程碑计划，含时间估算和验收标准 |
| [项目结构](./docs/PROJECT_STRUCTURE_CN.md) | 目录拓扑与模块架构说明 |
| [智能体设计原则](./AGENTS_CN.md) | 硬软节点协同、状态机流转、影子记忆智能体 |
| [设计推演记录](./docs/talk&thoughts/DESIGN_RATIONALE_CN.md) | 架构决策记录（对话形式） |

## 快速开始

*（项目当前处于设计/脚手架阶段。实施从 M1 Sprint 1 开始。）*

### 后端
```bash
cd backend
go mod tidy
make run
```

### 前端
```bash
cd frontend
npm install
npm run dev
```

### 全栈（Docker）
```bash
docker-compose up
```

## 项目进度

| 里程碑 | 状态 | 目标 |
|--------|------|------|
| M1：可运行的 MVP | 🔲 未开始 | 核心引擎 + 画布 + 硬/软节点 |
| M2：生产可用版 | 🔲 未开始 | MCP + 路由 + 认证 + 部署 |
| M3：差异化竞争力 | 🔲 未开始 | RAG 记忆 + 高级 UI |
| M4：实验性 | 🔲 未开始 | WebMCP 浏览器自动化 |

## 灵感与致谢

Synapse 的架构思想部分汲取自业内先进开源工程化设计（如 `claude-code` 对 MCP 工具动态挂载与记忆提取子智能体的设计，以及 `claw-code` 中有限状态机路由思想）。

## License

MIT License
