package extract

import "github.com/leporel/huetension/internal/color"

// Octree quantisation: insert every pixel into an 8-ary RGB tree (one bit per
// channel per level), then iteratively collapse the deepest internal node
// with the smallest cumulative pixel count until ≤ k leaves remain. Each
// surviving leaf becomes one palette color, weighted by its leaf count.
//
// Deterministic, memory-bounded by tree depth (~8 levels), and ~3-5× faster
// than k-means on typical images.

const octreeDepth = 8

type octNode struct {
	children [8]*octNode
	isLeaf   bool
	count    uint64
	rSum     uint64
	gSum     uint64
	bSum     uint64
}

func extractOctree(pixels []color.Color, k int) []color.Color {
	if len(pixels) == 0 {
		return nil
	}
	if k <= 0 {
		k = 1
	}

	root := &octNode{}
	leafCount := 0
	for _, p := range pixels {
		leafCount = octInsert(root, p, 0, leafCount)
	}

	// Phase 1: collapse whole subtrees as long as doing so doesn't push
	// leafCount below k. This is fast (O(tree)) and handles the common case.
	for leafCount > k {
		next := octReduce(root, leafCount, k)
		if next >= leafCount {
			break
		}
		leafCount = next
	}

	var nodes []*octNode
	octCollect(root, &nodes)

	// Phase 2: tree-collapse can't always land exactly at k (e.g. a single
	// internal node owning 8 leaves can only collapse to 1). Greedy
	// nearest-pair merging on the remaining leaves finishes the job.
	//
	// Distance is computed in OkLab (perceptually uniform) rather than raw
	// RGB so we don't merge two greens at different lightnesses just because
	// their RGB values happen to be close. We pre-project each leaf into
	// OkLab once, then merge in that space.
	leaves := make([]octLeaf, len(nodes))
	for i, n := range nodes {
		leaves[i] = newOctLeaf(n)
	}
	for len(leaves) > k {
		leaves = octMergeNearestLeafPair(leaves)
	}

	var total uint64
	for _, l := range leaves {
		total += l.count
	}

	out := make([]color.Color, 0, len(leaves))
	for _, l := range leaves {
		if l.count == 0 {
			continue
		}
		c := color.New(
			uint8(l.rSum/l.count),
			uint8(l.gSum/l.count),
			uint8(l.bSum/l.count),
		)
		if total > 0 {
			c.Freq = float64(l.count) / float64(total)
		}
		out = append(out, c)
	}
	return out
}

// octLeaf is the post-tree-collapse projection of an octNode: pixel sums
// for the centroid plus a precomputed OkLab triple so Phase 2 pair-merging
// can compare leaves perceptually without re-projecting on every distance
// check.
type octLeaf struct {
	count            uint64
	rSum, gSum, bSum uint64
	okL, okA, okB    float64
}

func newOctLeaf(n *octNode) octLeaf {
	leaf := octLeaf{
		count: n.count,
		rSum:  n.rSum,
		gSum:  n.gSum,
		bSum:  n.bSum,
	}
	if n.count > 0 {
		c := color.New(
			uint8(n.rSum/n.count),
			uint8(n.gSum/n.count),
			uint8(n.bSum/n.count),
		)
		leaf.okL, leaf.okA, leaf.okB = c.ToOkLab()
	}
	return leaf
}

// octInsert walks the tree to the appropriate leaf for c, accumulating count
// and color sums on every traversed node so any future collapse already has
// the running totals it needs.
func octInsert(n *octNode, c color.Color, level, leafCount int) int {
	n.count++
	n.rSum += uint64(c.R)
	n.gSum += uint64(c.G)
	n.bSum += uint64(c.B)

	if level == octreeDepth || n.isLeaf {
		if !n.isLeaf {
			n.isLeaf = true
			leafCount++
		}
		return leafCount
	}

	idx := octIndex(c, level)
	if n.children[idx] == nil {
		n.children[idx] = &octNode{}
	}
	return octInsert(n.children[idx], c, level+1, leafCount)
}

// octIndex computes the 3-bit child index for c at the given depth.
// Bit 2 = R, bit 1 = G, bit 0 = B (so the bits read MSB-first as we descend).
func octIndex(c color.Color, level int) int {
	shift := uint(7 - level)
	idx := 0
	if (c.R>>shift)&1 != 0 {
		idx |= 4
	}
	if (c.G>>shift)&1 != 0 {
		idx |= 2
	}
	if (c.B>>shift)&1 != 0 {
		idx |= 1
	}
	return idx
}

// octReduce promotes the deepest internal node with the smallest total
// pixel count to a leaf, collapsing its entire subtree into a single
// average color.
//
// Two constraints filter the candidate set:
//
//  1. leaves >= 2 — collapsing a single-leaf chain is net zero progress
//     (lose 1 leaf, gain 1) and would break the outer loop early.
//  2. leafCount - leaves + 1 >= k — don't overshoot. If the only candidate
//     would push us below k, return unchanged so the caller can fall back
//     to leaf-pair merging.
func octReduce(root *octNode, leafCount, k int) int {
	maxRemovable := leafCount - k + 1
	if maxRemovable < 2 {
		return leafCount
	}

	var best *octNode
	var bestLeaves int
	bestDepth := -1
	var bestCount uint64 = ^uint64(0)

	var walk func(n *octNode, depth int) int
	walk = func(n *octNode, depth int) int {
		if n == nil {
			return 0
		}
		if n.isLeaf {
			return 1
		}
		leaves := 0
		for _, c := range n.children {
			leaves += walk(c, depth+1)
		}
		if leaves >= 2 && leaves <= maxRemovable {
			if depth > bestDepth || (depth == bestDepth && n.count < bestCount) {
				best = n
				bestDepth = depth
				bestCount = n.count
				bestLeaves = leaves
			}
		}
		return leaves
	}
	walk(root, 0)
	if best == nil {
		return leafCount
	}

	for i := range best.children {
		best.children[i] = nil
	}
	best.isLeaf = true
	return leafCount - bestLeaves + 1
}

// octMergeNearestLeafPair finds the two leaves closest in OkLab and merges
// them into a single synthesised leaf — pixel sums add, OkLab triple is
// recomputed from the merged centroid so subsequent comparisons stay
// perceptually correct.
//
// O(n²) per call, but n is the surviving leaf count after tree collapse
// (typically ≤ a few hundred), and we call this at most n-k times. Plenty
// fast for palette sizes.
func octMergeNearestLeafPair(leaves []octLeaf) []octLeaf {
	if len(leaves) < 2 {
		return leaves
	}
	bestI, bestJ := 0, 1
	bestD := octLeafDistSq(leaves[0], leaves[1])
	for i := 0; i < len(leaves); i++ {
		for j := i + 1; j < len(leaves); j++ {
			d := octLeafDistSq(leaves[i], leaves[j])
			if d < bestD {
				bestD = d
				bestI, bestJ = i, j
			}
		}
	}
	a, b := leaves[bestI], leaves[bestJ]
	merged := octLeaf{
		count: a.count + b.count,
		rSum:  a.rSum + b.rSum,
		gSum:  a.gSum + b.gSum,
		bSum:  a.bSum + b.bSum,
	}
	if merged.count > 0 {
		c := color.New(
			uint8(merged.rSum/merged.count),
			uint8(merged.gSum/merged.count),
			uint8(merged.bSum/merged.count),
		)
		merged.okL, merged.okA, merged.okB = c.ToOkLab()
	}
	leaves = append(leaves[:bestJ], leaves[bestJ+1:]...)
	leaves[bestI] = merged
	return leaves
}

// octLeafDistSq is squared OkLab distance — perceptually uniform so two
// shades that look the same to a human end up close even if their RGB
// values aren't.
func octLeafDistSq(a, b octLeaf) float64 {
	dL := a.okL - b.okL
	da := a.okA - b.okA
	db := a.okB - b.okB
	return dL*dL + da*da + db*db
}

func octCollect(n *octNode, out *[]*octNode) {
	if n == nil {
		return
	}
	if n.isLeaf {
		*out = append(*out, n)
		return
	}
	for _, c := range n.children {
		octCollect(c, out)
	}
}
