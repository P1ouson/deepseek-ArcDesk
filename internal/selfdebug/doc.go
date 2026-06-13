// Package selfdebug orchestrates the P0 write→verify→observe→analyze→fix loop.
// It unifies dependency, callgraph, runtime, and verification retry context into
// one host-driven cycle and surfaces immediate hints when verify commands fail.
package selfdebug
