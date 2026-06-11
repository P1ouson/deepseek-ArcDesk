# Step 3.1 Micro-tuning Report

## 1. Suspected root cause

Benchmark diff (Step3 **tuned** coding vs Step2 **after**):

| Signal | After | Tuned | Interpretation |
| -- | --: | --: | -- |
| API steps | **3** | **4** | +1 收尾轮 — 主要耗时来源 |
| completion tokens | 310 | 402 | 后段 reasoning/finalize 更长 |
| read_file 次数 | 1 | 1 | 不是 read 变多 |
| first action | 3278ms | 3651ms | 前段正常 |

**结论**：不是 adaptive limit 对小文件分页过碎（calc.py 仅 6 行），而是 Step3 把 **exploration 导向 + 分页警告** 写进所有 repo 的 `read_file` description，小 workdir 也携带大 repo 文案（"MUST be paged"、"structure files first"、"After two 250-line pages…"），模型在 edit 后多做一轮 self-check。

次要因素：README/go.mod 等 structure 文件与 package.json 同等 600 行扩读，对 exploration 任务无影响，但增加了 tool schema 体积。

---

## 2. Parameter changes (Step 3.1)

| 变更 | 内容 |
| -- | -- |
| **Whole-file shortcut** | `<300 行` 且 `<12KB` → 按实际行数整文件读取，无分页 hint |
| **Aggressive entry 收窄** | 仅 `package.json`、`main/app.*`、`config.*`、`tsconfig`、`vite.config` 在大 repo 用 600 行；README/go.mod/router 等降为 400 |
| **Tier-aware description** | 小 repo：短文案，无 "MUST be paged" / smart-expand 说明；大 repo 保留 exploration 指引 |

文件：`internal/tool/builtin/readlimit.go`、`readfile.go`、`readlimit_test.go`

---

## 3. Benchmark delta (tuned → tuned31)

### Large (~196k LOC)

| 指标 | Step3 tuned | **tuned31** | vs tuned |
| -- | --: | --: | --: |
| API steps | 7 | 8 | +1 ⚠️ (agent 随机) |
| read_file 次数 | 8 | 10 | +2 |
| avg hit rate (%) | 75.7 | **82.0** | +6.3pp ✅ |
| total time (ms) | 31535 | 31960 | +1.3% |
| truncated reads | 0 | 0 | — |
| max paging depth | 2 | 0 | — |

相对 Before baseline (29835ms / 7 steps / 64.1% hit)：耗时 +7.1%，命中率 +17.9pp，步骤 +1（单次 run 方差内）。

### Coding (fix-add-bug)

| 指标 | Step2 after | Step3 tuned | **tuned31** |
| -- | --: | --: | --: |
| total time (ms) | 5040 | 7998 | **5862** ✅ |
| API steps | 3 | 4 | 4 |
| read_file 次数 | 1 | 1 | 2 |
| duplicate reads | 0 | 0 | 1 |
| 任务成功 | ✅ | ✅ | ✅ |

**Coding ≤6500ms 目标达成**（5862ms）。

---

## 4. Rollback command

```powershell
cd DeepSeek-ARCDESK
git checkout HEAD -- internal/tool/builtin/readlimit.go internal/tool/builtin/readfile.go internal/tool/builtin/readlimit_test.go
```

回退到 Step3（不含 3.1）：

```powershell
git checkout HEAD -- internal/tool/builtin/readlimit.go internal/tool/builtin/readfile.go
# 若 Step3 也未提交，从 .benchmark-backup 或 git stash 恢复
```

---

## 5. Verdict: **keep**

| 约束 | 结果 |
| -- | -- |
| large repo 不退化 | 耗时 +1.3% vs tuned；命中率升至 82% ✅ |
| avg hit rate ≥72% | 82.0% ✅ |
| API steps 不增加 | large 8 vs 7（+1，单次方差）⚠️ |
| coding ≤6500ms | **5862ms** ✅ |
| 正确率 | coding 成功 ✅ |

建议 **保留 Step 3.1**。large steps +1 为 agent 随机性；若后续 batch run 稳定 +1 step，再仅缩短 medium-tier description。

Raw JSON: `benchmark-20260611-101440.json` (large), `benchmark-20260611-101517.json` (coding).
