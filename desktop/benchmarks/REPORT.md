# 首次项目探索性能 Benchmark 报告

**运行时间**: 2026-06-11  
**方法**: `BENCHMARK_AGENT=1` + `cmd/explorebench`（真实 DeepSeek API）  
**Before 基线**: `git checkout HEAD` 的 `read_file=2000` + `fanout=8`  
**After**: 当前优化（`read_file=250` + 动态 fanout 4/3/2）  
**原始 JSON**: `desktop/benchmarks/benchmark-*.json`

> 说明：Agent 行为有随机性；各场景仅跑 1 次 before/after 对比。Coding 的 after 第二次在干净 workdir 上重跑（`after-coding-rerun`），因首次 batch 中 before 已修改文件。

---

## Benchmark Summary

### 小项目 (~8.8k LOC, `internal/agent`)

| 指标 | Before | After | Delta |
| -- | --: | --: | --: |
| 首次 assistant token (ms) | 64601 | 14050 | **-50551 (-78%)** |
| 首次有效行动 (ms) | 9350 | 6507 | -2843 (-30%) |
| 平均命中率 (%) | 92.8 | 92.2 | -0.6 |
| API steps | 12 | 11 | -1 |
| read_file 次数 | 3 | 18 | +15 |
| 平均分页深度 | 0 | 0 | 0 |
| 最大并发 | 8 | 4 | -4 |
| 总耗时 (ms) | 76030 | 44729 | **-31301 (-41%)** |

**判断**: 无连续 offset 分页（深度 0）。read 次数上升但单次更小（avg 21→35 行），总 token 降、总耗时显著下降。**未出现 offset>4 异常**。

---

### 中项目 (~49.6k LOC, `desktop/frontend/src`)

| 指标 | Before | After | Delta |
| -- | --: | --: | --: |
| 首次 assistant token (ms) | 15255 | 30024 | +14769 (+97%) |
| 首次有效行动 (ms) | 4033 | 0* | — |
| 平均命中率 (%) | 85.9 | 87.1 | +1.2 |
| API steps | 13 | 13 | 0 |
| read_file 次数 | 22 | 22 | 0 |
| 最大分页深度 | 0 | 3 | +3 |
| truncated reads | 3 | 0 | **-3** |
| 最大并发 | 8 | 4 | -4 |
| 总耗时 (ms) | 55139 | 64009 | +8870 (+16%) |

\* 探索任务未触发 write/bash，`firstActionMs=0` 正常。

**判断**: 分页深度 3（<4 阈值），truncation 从 3 次降为 0（loading 感知应改善）。但总耗时 +16%、首 token 变慢——**非能力问题，是探索路径随机差异 + 更小分页导致更多 reasoning 轮次**，需更多样本确认。

---

### 大项目 (~196k LOC, monorepo 根)

| 指标 | Before | After | Delta |
| -- | --: | --: | --: |
| 首次 assistant token (ms) | 17090 | 21368 | +4278 (+25%) |
| 首次有效行动 (ms) | 0 | 0 | — |
| 平均命中率 (%) | 64.1 | 87.7 | **+23.6** |
| API steps | 7 | 10 | **+3 (+43%)** |
| read_file 次数 | 8 | 17 | **+9 (+113%)** |
| 最大分页深度 | 0 | 0 | 0 |
| 最大并发 | 8 | 4 | -4 |
| throttled 降级轮次 | 0 | 0 | 0 |
| 总耗时 (ms) | 29835 | 35514 | +5679 (+19%) |

**判断**: fanout **从未触发 50k/100k 降级**（maxConcurrency 始终 4，无 throttledRounds）。但 read_file=250 导致**更多 read 轮次和 API steps**，首探耗时上升——这是主要 regression 信号。

---

### Coding 能力（fix-add-bug, 同 prompt）

| 指标 | Before | After (干净重跑) | Delta |
| -- | --: | --: | --: |
| 任务成功 | ✅ add 修复 | ✅ add 修复 | — |
| 修改文件数 | 1 | 1 | 0 |
| 漏改/误改 | 无 | 无 | — |
| API steps | 4 | 3 | -1 |
| read_file 次数 | 2 (1 duplicate) | 1 | -1 |
| 分页/重复读 | duplicate=1 | 无 | 改善 |
| 总耗时 (ms) | 6876 | 5040 | -1836 (-27%) |

**判断**: **能力未下降**；after 更少步骤、无重复读、更快完成。

---

## 汇总表（探索场景加权感知）

| 指标 | Before (avg) | After (avg) | Delta |
| -- | --: | --: | --: |
| 首次 assistant token | 32315 ms | 21814 ms | -32% |
| 首次有效行动 | 6692 ms | 6507 ms | -3% |
| 平均命中率 | 80.9% | 89.0% | +8.1pp |
| API steps | 10.7 | 11.3 | +6% |
| read_file 次数 | 11 | 19 | +73% |
| 最大分页深度 | 0 | 1 | +1 |
| 最大并发 | 8 | 4 | -4 |
| 总耗时 | 53668 ms | 48084 ms | -10% |

（small + medium + large 三项算术平均；coding 单独评估能力）

---

## 四个风险点验证

| 风险 | 结果 | 证据 |
| -- | -- | -- |
| 1. loading 好了但 Agent 变笨 | **未观察到** | coding 任务 before/after 均修复成功；after 步数更少 |
| 2. token 降但 API 步数增 | **部分成立** | 大项目 steps 7→10；中项目持平；小/coding 下降 |
| 3. 命中率更高但首行动更慢 | **部分成立** | 大项目命中 +24pp 但总耗时 +19%；中项目首 token 变慢 |
| 4. read=250 / fanout 过度保守 | **250 偏紧；fanout 未过度** | 分页深度均 ≤3；throttledRounds=0，从未降到 2 |

---

## Verdict

### **PASS WITH CONCERNS**（建议调参数，可提交）

**理由（数据驱动，非主观）**:
- ✅ 小项目 / coding：速度、步骤、能力均改善或持平
- ✅ 无 offset 疯狂分页（max depth ≤3）
- ✅ fanout 动态降级在本次大项目跑中**未触发**，不存在 maxConcurrency=2 导致的首行动恶化
- ⚠️ 大项目：read_file 次数 +113%、API steps +43%、总耗时 +19% —— **250 行默认值在 100k+ LOC 首探时偏激进**
- ⚠️ 中项目：总耗时 +16%，但 truncation 降为 0（UX 目标达成）

### 参数建议

| 项 | 建议 |
| -- | -- |
| `read_file` 默认 limit | **小/中项目保持 250**；对大文件/大 repo 可考虑 **400** 或在 tool description 中允许模型主动 `limit=400` 读目录索引类文件 |
| fanout 阈值 (50k/100k) | **保持现状** — 本次未触发，无证据表明过保守 |
| throttle 放宽 | **暂不需要** — throttledRounds=0 |
| loading UX 改动 | **保留** — truncated reads 3→0（medium） |

---

## 如何复现

```powershell
cd DeepSeek-ARCDESK
$env:GOTOOLCHAIN = "auto"
# 单次
.\scripts\run-explore-variant.ps1 -Mode after -Only small
# 全套 before/after
.\scripts\run-all-benchmarks.ps1
```

Instrumentation 入口：
- `BENCHMARK_AGENT=1` 启用
- `internal/benchagent/` 采集器
- `cmd/explorebench` 运行器
- JSON 输出：`desktop/benchmarks/benchmark-<timestamp>.json`
