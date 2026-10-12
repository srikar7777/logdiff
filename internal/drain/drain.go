// Package drain implements the Drain3 log clustering algorithm.
// Reference: He et al., "Drain: An Online Log Parsing Approach with Fixed Depth Tree"
package drain

import (
	"fmt"
	"sort"
	"sync/atomic"
)

// Wildcard is the placeholder token used in log templates where a position
// contains different values across log lines.
const Wildcard = "<*>"

// Config controls the Drain clustering behaviour.
type Config struct {
	// MaxDepth is the depth of the prefix tree (default 4).
	MaxDepth int
	// SimThreshold is the minimum ratio of matching tokens to merge a line
	// into an existing cluster (default 0.4).
	SimThreshold float64
	// MaxChildren is the maximum fan-out per internal node (default 100).
	MaxChildren int
}

func applyDefaults(c Config) Config {
	if c.MaxDepth <= 0 {
		c.MaxDepth = 3 // branch on first 3 tokens (after length)
	}
	if c.SimThreshold <= 0 {
		c.SimThreshold = 0.5 // higher threshold for more precise matching
	}
	if c.MaxChildren <= 0 {
		c.MaxChildren = 100
	}
	return c
}

// LogCluster represents a group of log lines that share a common template.
type LogCluster struct {
	ID       string
	Template []string
	Count    int64
}

// prefixNode is a node in the fixed-depth prefix tree.
type prefixNode struct {
	children map[string]*prefixNode
	clusters []*LogCluster
}

func newNode() *prefixNode {
	return &prefixNode{children: make(map[string]*prefixNode)}
}

// Drain mines log templates using the Drain3 fixed-depth prefix-tree algorithm.
type Drain struct {
	cfg    Config
	root   *prefixNode
	byID   map[string]*LogCluster
	nextID uint64
}

// New creates a Drain instance with the given configuration.
func New(cfg Config) *Drain {
	return &Drain{
		cfg:  applyDefaults(cfg),
		root: newNode(),
		byID: make(map[string]*LogCluster),
	}
}

// Add processes a tokenised log line, finds the best matching cluster or
// creates a new one, and returns the cluster.
func (d *Drain) Add(tokens []string) *LogCluster {
	if len(tokens) == 0 {
		return nil
	}
	if c := d.treeSearch(tokens); c != nil {
		d.merge(c, tokens)
		return c
	}
	return d.insert(tokens)
}

// Clusters returns all clusters sorted by Count descending.
func (d *Drain) Clusters() []*LogCluster {
	out := make([]*LogCluster, 0, len(d.byID))
	for _, c := range d.byID {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		return atomic.LoadInt64(&out[i].Count) > atomic.LoadInt64(&out[j].Count)
	})
	return out
}

// ─── internal helpers ──────────────────────────────────────────────────────

// treeSearch walks the prefix tree and returns the best matching cluster,
// or nil if no cluster meets the similarity threshold.
func (d *Drain) treeSearch(tokens []string) *LogCluster {
	node := d.root

	// Level 1: branch by token count
	lenKey := fmt.Sprintf("len:%d", len(tokens))
	child, ok := node.children[lenKey]
	if !ok {
		return nil
	}
	node = child

	// Levels 2..MaxDepth: branch by concrete prefix tokens
	for depth := 1; depth < d.cfg.MaxDepth; depth++ {
		idx := depth - 1
		if idx >= len(tokens) {
			break
		}
		tok := tokens[idx]
		if next, ok := node.children[tok]; ok {
			node = next
		} else if next, ok := node.children[Wildcard]; ok {
			node = next
		} else {
			return nil
		}
	}
	return d.bestMatch(node.clusters, tokens)
}

// bestMatch finds the cluster in candidates with the highest similarity to
// tokens, returning it only if the score meets SimThreshold.
func (d *Drain) bestMatch(candidates []*LogCluster, tokens []string) *LogCluster {
	var best *LogCluster
	var bestScore float64
	for _, c := range candidates {
		if len(c.Template) != len(tokens) {
			continue
		}
		if s := similarity(c.Template, tokens); s > bestScore {
			bestScore, best = s, c
		}
	}
	if bestScore >= d.cfg.SimThreshold {
		return best
	}
	return nil
}

// insert creates a new cluster and adds it to the tree and registry.
func (d *Drain) insert(tokens []string) *LogCluster {
	d.nextID++
	c := &LogCluster{
		ID:       fmt.Sprintf("c%d", d.nextID),
		Template: append([]string(nil), tokens...),
		Count:    1,
	}
	d.byID[c.ID] = c

	node := d.root

	// Level 1: length bucket
	lenKey := fmt.Sprintf("len:%d", len(tokens))
	if _, ok := node.children[lenKey]; !ok {
		node.children[lenKey] = newNode()
	}
	node = node.children[lenKey]

	// Levels 2..MaxDepth: token prefix
	for depth := 1; depth < d.cfg.MaxDepth; depth++ {
		idx := depth - 1
		if idx >= len(tokens) {
			break
		}
		tok := tokens[idx]
		if _, ok := node.children[tok]; !ok {
			if len(node.children) >= d.cfg.MaxChildren {
				tok = Wildcard
			}
		}
		if _, ok := node.children[tok]; !ok {
			node.children[tok] = newNode()
		}
		node = node.children[tok]
	}
	node.clusters = append(node.clusters, c)
	return c
}

// merge increments cluster count and widens any template positions where the
// incoming token differs from the stored template token.
func (d *Drain) merge(c *LogCluster, tokens []string) {
	atomic.AddInt64(&c.Count, 1)
	for i, tmpl := range c.Template {
		if i >= len(tokens) {
			break
		}
		if tmpl != Wildcard && tmpl != tokens[i] {
			c.Template[i] = Wildcard
		}
	}
}

// similarity returns (matching positions) / (total positions).
// Wildcard positions in the template always count as matching.
func similarity(tmpl, tokens []string) float64 {
	if len(tmpl) == 0 {
		return 0
	}
	var match int
	for i, t := range tmpl {
		if i >= len(tokens) {
			break
		}
		if t == Wildcard || t == tokens[i] {
			match++
		}
	}
	return float64(match) / float64(len(tmpl))
}
