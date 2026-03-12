# C++ → Go 大型项目重构工作流

本文档描述使用 GoClaw 公司化蜂群模式（Corporate Swarm）将大型 C++ 项目重构为 Go 的完整工作流程。

## 团队架构

```
用户 (IM/CLI)
    |
Secretary (项目协调员) ←→ 用户唯一对接窗口
    |
    ├── HR (资源评估师) — 分析复杂度、分配 Worker、制定方案
    │
    └── PM (项目经理) — 拆分任务、调度执行、追踪进度
         |
         ├── Analyst ×2    — C++ 源码分析（Phase 0）
         ├── Researcher ×1 — Go 生态调研（Phase 1）
         ├── Architect ×1  — Go 架构设计（Phase 1）
         ├── Coder ×3      — 代码移植（Phase 2-3）
         ├── Tester ×1     — 测试编写（Phase 2-4）
         ├── Reviewer ×1   — 代码审查（Phase 2-4）
         └── DocWriter ×1  — 文档编写（Phase 4-5）
```

## Worker 角色说明

| 角色 | 模型 | 允许工具 | 职责 |
|------|------|----------|------|
| Analyst | opus | read_file, list_dir, run_shell, memory_search, agent_call | C++ 源码结构和风险分析 |
| Architect | opus | read_file, write_file, list_dir, web_search, web_fetch, memory_search, agent_call | Go 架构设计和接口定义 |
| Coder | sonnet | read_file, write_file, edit_file, list_dir, run_shell, memory_search, agent_call | C++ → Go 代码移植 |
| Reviewer | opus | read_file, list_dir, run_shell, memory_search, agent_call | 代码正确性和惯用性审查 |
| Tester | sonnet | read_file, write_file, edit_file, list_dir, run_shell, memory_search, agent_call | 单元测试、集成测试、基准测试 |
| DocWriter | sonnet | read_file, write_file, list_dir, memory_search, agent_call | API 文档、迁移指南、README |
| Researcher | opus | web_search, web_fetch, read_file, write_file, memory_search, agent_call | Go 生态替代方案调研 |

## 六阶段工作流

### Phase 0：源码分析（Analyst ×2）

**目标**：全面理解 C++ 项目的结构、规模和风险。

**任务清单**：
- [ ] 扫描 C++ 项目目录结构
- [ ] 统计代码规模（行数、文件数、头文件/实现文件比例）
- [ ] 分析头文件 `#include` 依赖图
- [ ] 识别类继承层级（单继承、多继承、虚继承）
- [ ] 识别 C++ 特有模式（模板元编程、SFINAE、CRTP、运算符重载等）
- [ ] 标注第三方库依赖
- [ ] 为每个模块标注移植难度等级（低/中/高/极高）

**产出**：
- `analysis_report.json` — 项目结构和规模数据
- `api_inventory.md` — 公共 API 清单
- `risk_assessment.md` — 模块级风险评估矩阵

**完成标准**：所有模块已分析并标注难度等级。

---

### Phase 1：架构设计（Architect ×1 + Researcher ×1）

**目标**：设计 Go 项目架构，找到所有 C++ 依赖的 Go 替代方案。

**任务清单**：
- [ ] Researcher 调研 Go 生态替代方案（Boost.Asio → net 等）
- [ ] Architect 将 C++ namespace/class 映射到 Go package/struct
- [ ] 定义 Go 接口边界和包结构
- [ ] 设计并发模型（goroutine + channel）
- [ ] 设计错误处理策略
- [ ] 编写接口骨架 .go 文件

**产出**：
- `architecture_mapping.md` — C++ → Go 架构对照表
- `go_package_layout.md` — Go 目录和包结构
- `interfaces.go` — 核心接口骨架
- `research_report.md` — Go 生态替代方案调研报告

**完成标准**：架构设计通过 Reviewer 审查，用户审批通过。

---

### Phase 2：核心层移植（Coder ×3 + Tester ×1）

**目标**：移植底层工具库和核心模块。

**Batch 1（并行）— 工具层**：
- [ ] 日志模块
- [ ] 配置模块
- [ ] 序列化模块
- [ ] 工具函数库

**Batch 2（串行依赖 Batch 1）— 核心层**：
- [ ] 事件循环
- [ ] 定时器
- [ ] 缓冲区管理

**每个 Batch 完成后的质量关卡**：
- [ ] `go build` 通过
- [ ] `go test` 通过（覆盖率 ≥ 80%）
- [ ] `go vet` 无警告
- [ ] Reviewer 审查通过（无 blocker）

**产出**：Go 源码 + 测试 + `porting_notes.md`

---

### Phase 3：上层模块移植（Coder ×3 + Tester ×1 + Reviewer ×1）

**目标**：移植网络层和应用层模块。

**Batch 3（串行依赖 Batch 2）— 网络层**：
- [ ] TCP 模块
- [ ] UDP 模块
- [ ] SSL/TLS 模块

**Batch 4（串行依赖 Batch 3）— HTTP 层**：
- [ ] HTTP Server
- [ ] HTTP Client
- [ ] Router / Middleware

**产出**：Go 源码 + 集成测试 + 性能基准

---

### Phase 4：质量保障（Reviewer + Tester + DocWriter）

**目标**：全项目质量审查和文档编写。

**任务清单**：
- [ ] 全项目代码审查
- [ ] 覆盖率分析（目标 ≥ 80%）
- [ ] 性能对比（Go vs C++ 基准测试）
- [ ] API 文档编写（godoc 注释）
- [ ] 迁移指南编写

**产出**：
- 审查报告
- 覆盖率报告
- 性能对比报告
- API 文档

---

### Phase 5：收尾整合（DocWriter ×1）

**目标**：完成项目文档和 CI/CD 配置。

**任务清单**：
- [ ] 编写项目 README
- [ ] 编写 CHANGELOG
- [ ] 编写迁移指南
- [ ] 编写示例代码
- [ ] 配置 CI/CD（Makefile + GitHub Actions）

**产出**：完整的可发布项目。

---

## Agent 间协作消息流程

### 启动流程
```
用户 → Secretary: "请将 xxx C++ 项目重构为 Go"
Secretary → HR: "用户需要将 C++ 项目重构为 Go，请评估资源需求"
HR → Secretary: "提交审批方案（6 阶段计划、Worker 分配）"
Secretary → 用户: "转述审批方案"
用户 → Secretary: "批准"
Secretary → HR: "用户已批准"
HR → PM: "执行重构计划，以下是 Worker 分配..."
```

### Phase 0 执行流程
```
PM → Analyst-1: "分析 src/core/ 和 src/utils/ 目录"
PM → Analyst-2: "分析 src/net/ 和 src/http/ 目录"
Analyst-1 → PM: "core 分析完成：3 个高风险模块..."
Analyst-2 → PM: "net 分析完成：模板密集区域..."
PM → Secretary: "Phase 0 完成，发现 X 个模块，Y 个高风险"
Secretary → 用户: "阶段 1/6 完成：源码分析结果..."
```

### Phase 2-3 执行流程
```
PM → Coder-1: "移植 logger 包（对照 architecture_mapping.md）"
PM → Coder-2: "移植 config 包"
PM → Coder-3: "移植 serializer 包"
PM → Tester: "为 logger 包编写测试"
Coder-1 → PM: "logger 包移植完成"
PM → Reviewer: "审查 logger 包"
Reviewer → PM: "logger 审查通过 / 发现 2 个 warning"
PM → Secretary: "Batch 1 完成，质量关卡通过"
```

## 配置文件

蜂群配置文件位于 `examples/swarm/cpp2go-refactor.json`，使用方式：

```bash
goclaw swarm start --config examples/swarm/cpp2go-refactor.json
```

## C++ → Go 转换规则速查表

| C++ 概念 | Go 等价物 | 关键点 |
|----------|----------|--------|
| RAII / 析构函数 | `defer resource.Close()` | 所有 io.Closer 必须 defer 关闭 |
| `unique_ptr/shared_ptr` | 普通指针 `*T`，GC 回收 | 禁止 unsafe.Pointer |
| 模板 `template<T>` | 泛型 `[T any]` | 简单场景优先用接口 |
| 模板元编程/SFINAE | 不适用 | 改为运行时计算或代码生成 |
| 单继承 | 结构体嵌入 | 组合优于继承 |
| 虚函数/抽象类 | `interface` | 隐式实现，小接口原则 |
| 多重继承 | 多接口组合 | 嵌入 + 接口 |
| `throw/catch` | `return error` | 永远不用 panic 替代 error |
| `std::thread` | `go func(){}()` | goroutine-per-task |
| `std::mutex` | `sync.Mutex` | `mu.Lock(); defer mu.Unlock()` |
| `condition_variable` | channel | 优先用 channel 通信 |
| `enum class` | `type X int` + `iota` | 常量组 |
| 运算符重载 | 方法（Equal/Less/String） | Go 不支持运算符重载 |
| `#ifdef` | `//go:build` 标签 | 构建标签 |
| `namespace` | `package` | 目录即包 |
| `std::vector` | `[]T`（切片） | 注意 append 的扩容语义 |
| `std::map` | `map[K]V` | 并发安全需 sync.Map 或加锁 |
| `std::optional` | `*T` 或 ok 模式 | `val, ok := m[key]` |
| `const&` 传参 | 值传参或指针 | 小结构体传值，大结构体传指针 |
| 头文件 `.h/.hpp` | 无（大写导出） | 首字母大写 = public |
| `static` 成员 | 包级变量/函数 | 无 class 级 static |
| 友元 `friend` | 同包可见 | 小写字段同包可访问 |
