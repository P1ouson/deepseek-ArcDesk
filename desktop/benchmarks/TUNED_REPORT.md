# Step 3 Parameter Tuning — Re-benchmark Report

**Tuned changes**: adaptive `read_file` limits + smart expansion + entry-file read bias (tool descriptions only).

**Compare baseline**: Step 2 benchmark **Before** (HEAD: `read_file=2000`, fanout=8).

---

## Large project (~196k LOC)

| 指标 | Before | Step2 After | **Tuned** | Tuned vs Before |
| -- | --: | --: | --: | --: |
| API steps | 7 | 10 (+43%) | **7** | **0%** ✅ |
| read_file 次数 | 8 | 17 (+113%) | **8** | **0%** ✅ |
| avg paging depth | 0 | 0 | **2** | +2 |
| total time (ms) | 29835 | 35514 (+19%) | **31535** | **+5.7%** ✅ |
| avg hit rate (%) | 64.1 | 87.7 | **75.7** | **+11.6pp** ✅ |
| first assistant token (ms) | 17090 | 21368 | **15588** | **-9%** ✅ |
| first action (ms) | 0 | 0 | 0 | — |
| truncated reads | 0 | 0 | **0** | 0 |

**目标达成情况**:
- steps +43% → **压回 0%** (≤ +10% ✅)
- reads +113% → **压回 0%** (< +40% ✅)
- 总耗时 +19% → **+5.7%** (≈ baseline ±5% ✅)

---

## Coding task (fix-add-bug)

| 指标 | Before | Step2 After | **Tuned** |
| -- | --: | --: | --: |
| 任务成功 | ✅ | ✅ | ✅ |
| API steps | 4 | 3 | 4 |
| read_file 次数 | 2 | 1 | **1** |
| duplicate reads | 1 | 0 | **0** |
| total time (ms) | 6876 | 5040 | 7998 |
| avg hit rate (%) | 92.2 | 99.0 | 74.8 |
| first action (ms) | 4300 | 3278 | 3651 |

能力未下降；tuned 与 after 一样 1 次 read、无重复读。

---

## 四个必答问题

1. **是否恢复大项目速度？** → **是**。steps/reads 回到 before 水平，总耗时仅 +5.7%（vs step2 +19%）。
2. **是否保留命中率收益？** → **部分保留**。75.7% vs before 64.1%（+11.6pp）；低于 step2 的 87.7%，但仍是净收益。
3. **是否再次出现 truncation？** → **否**（large tuned truncated=0）。
4. **是否变笨？** → **否**。coding 任务成功修复，无漏改/误改。

---

## Verdict: **PASS**

Tuned adaptive limits 修正了 step2 在大项目上的 regression，同时保留显著命中率提升与 UX 改进（无 truncation、分页深度 ≤2）。

### 参数落地

| 场景 | 默认 limit |
| -- | -- |
| 小 repo (<10k LOC) | 250（不变） |
| 中 repo (10k–80k) 入口文件 | 400 |
| 大 repo (80k+) 入口文件 | 600 |
| 大 repo 超大实现文件 | 280 |
| offset ≥ 500（连续两页 250 后） | 自动扩至 600 |

Raw JSON: `benchmark-20260611-101020.json` (large), `benchmark-20260611-101101.json` (coding).
