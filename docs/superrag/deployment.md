# SuperRAG 部署与使用

## 项目来源

本仓库从本地下载的 WeKnora 0.8.3 源码包建立基线，再叠加已验证的本地修改。源代码和上游版权声明保留，来源及源码包指纹见 [source-snapshot.json](source-snapshot.json)。仓库没有包含已有知识库的数据卷、数据库、模型凭证或微信登录状态。

## Docker 部署

需要 Docker Engine / Docker Desktop 和支持 `!override`、`!reset` 的 Docker Compose v2.24.4 或更新版本。后端包含 Go、Rust/Anydoc 和 BrowserSkill 构建步骤，首次构建需要网络访问和足够的磁盘空间；前端也从本仓库源码构建。

在新的部署环境中：

```powershell
git clone https://github.com/gumorty/SuperRAG.git
cd SuperRAG
Copy-Item .env.example .env
```

编辑本地 `.env`。以下是本次路线使用的非敏感选项，密码和加密密钥应填写自己的值：

```dotenv
COMPOSE_PROJECT_NAME=superrag
WEKNORA_VERSION=v0.8.3
FRONTEND_PORT=18088
APP_PORT=18080
DB_DRIVER=postgres
RETRIEVE_DRIVER=postgres
COMPOSE_PROFILES=neo4j
NEO4J_ENABLE=true
NEO4J_URI=bolt://neo4j:7687
NEO4J_USERNAME=neo4j
LLM_DEBUG_LOG=false
```

设置 `.env.example` 中的数据库、Redis、Neo4j 密码及 JWT/AES 等必需配置。Neo4j 的应用连接密码与数据库启动密码共用 `NEO4J_PASSWORD`。密钥不要放进智能体提示词或提交到 Git。

```powershell
docker compose config --quiet
docker compose build app frontend
docker compose up -d --wait --wait-timeout 300
docker compose ps
```

访问 <http://127.0.0.1:18088>。`docker-compose.override.yml` 使用本地构建镜像 `superrag-app:0.8.3-local` 和 `superrag-ui:0.8.3-local`，无需从镜像仓库寻找本机曾使用的临时镜像。

保留的上游 Compose 文件仍使用 `WeKnora-*` 固定容器名。已有 WeKnora 部署的同一 Docker 主机上不要直接再启动一套；需先为第二套部署单独配置容器名、端口和数据卷，或者使用另一台主机。仅克隆仓库和构建镜像不会迁移或修改现有数据。

`weknora.ps1` 可用于已配置并完成构建后的 `status`、`start`、`stop`、`restart`、`logs` 操作。构建和正常启动不应使用 `docker compose down -v`，以免删除数据卷。

## 模型与智能体

在模型管理中配置自己的硅基流动凭证。当前测试使用的模型为：

| 用途 | 模型 | 配置 |
|---|---|---|
| 对话 | `deepseek-ai/DeepSeek-V4.1-Flash` | 关闭思考，温度 0.1 |
| 向量 | `Qwen/Qwen3-Embedding-4B` | 1024 维 |
| 重排 | `Qwen/Qwen3-Reranker-4B` | 同一供应商 |
| 视觉 | `Qwen/Qwen3-VL-8B-Instruct` | 按需配置 |
| 语音识别 | `FunAudioLLM/SenseVoiceSmall` | 按需配置 |

Base URL：`https://api.siliconflow.cn/v1`。以上为本次实测配置，不保证所有账户未来均可调用；以自己的账号权限和供应商模型列表为准。

创建知识库并启用需要的 Wiki、图谱，等待离线处理完成。图谱还要求 Neo4j 服务和上述环境开关正常。模型、知识库及微信绑定信息存放在数据库，上传源码不会自动复制这些配置。

[智能体模板](../../config/examples/superrag-research-agent.json) 是 `POST /api/v1/agents` 的请求体模板。先把 `REPLACE_WITH_CHAT_MODEL_ID`、`REPLACE_WITH_RERANK_MODEL_ID` 和 `REPLACE_WITH_KNOWLEDGE_BASE_ID` 替换成当前实例实际 ID，再通过已认证 API 创建；也可在界面手工配置同样的提示词和工具。

该配置使用原文检索、原文读取、Wiki 搜索/阅读和图谱查询；候选数 50、重排 TopK 10。为隔离论文证据，测评配置关闭联网搜索、长期记忆和多模态上传；这不是对平台这些能力的删除。日常联网、微信和多模态助手应单独按用途配置。

新建对话时选择这个智能体。智能体工具链使用 `/api/v1/agent-chat/{session_id}`；固定知识问答接口和旧会话不会仅因为新增智能体就自动切换策略。`knowledge_base_ids` 与 `knowledge_ids` 的范围是并集，测试单篇文档时只指定该文档，避免同时选整库。

## 运行诊断

```powershell
docker compose exec app /app/rag-queue-control -action status
docker compose logs --tail 100 app
```

镜像包含 `graph-backfill` 和 `rag-queue-control`。它们是运维工具，不应在没有核对任务清单、活动任务和父任务计数时执行恢复或补建；重复提交可能重复计数或额外消耗模型额度。

完整模型请求日志默认关闭。临时验收启用后应及时归档到私有位置并关闭，原始文档和模型上下文不属于公开报告。

## 发布与验证

上游的 Docker Hub、PyPI、npm 和发行版发布任务已限制为腾讯上游仓库执行，避免本仓库推送时沿用腾讯的发布目标。常规 CI 文件保留；首次大规模源码导入使用 `[skip ci]`，不声称已完成 GitHub 全量 CI。

发布前已对本地对应代码执行 `internal/types`、`chat_pipeline`、`modelcontext` 的 Go 测试，并检查两个新增运维命令可编译。完整上游 Dockerfile 的冷构建尚未在本次上传流程重新执行；当前运行部署使用的本地镜像已经过实际 RAG 验证。
