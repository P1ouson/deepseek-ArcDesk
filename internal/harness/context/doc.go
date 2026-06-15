// Package context documents Harness layer 3: cache-stable prefix assembly and
// compression. Implementation lives in boot (prefix segments), control.Compose
// (turn-tail), agent.compact (session compaction), and ctxcompress (tool output).
//
// Formal segments (prefix, boot order):
//  1. base system prompt
//  2. output style
//  3. language policy
//  4. semantic memory block
//  5. repo map (reporag)
//  6. skills index
//  7. environment snapshot (envaware)
//
// Compression tiers (turn/session):
//  L1 tool output truncate (ctxcompress)
//  L2 log digest lines (ctxcompress)
//  L3 soft context notice (agent softCompactRatio)
//  L4 session compaction (agent compact)
//  L5 compact stuck halt (agent compactStuck)
package context

const PackageDoc = "harness layer 3 — see internal/harness/context/doc.go"
