package extract

import (
	"sort"

	"github.com/leporel/huetension/internal/color"
)

// DBSCAN palette extraction: each pixel is a point in OkLab space (×100 so
// distances read like ΔE units), pre-binned to a coarse grid for speed.
// Each occupied cell carries weight = pixel count; DBSCAN walks density-
// reachable cells into clusters.
//
// Two failure modes the implementation guards against:
//
//  1. Gradient chaining. Natural photos have continuous color gradients,
//     so DBSCAN at typical ε often merges everything into one giant
//     cluster. When that leaves us with fewer clusters than k, we pad
//     with farthest-point sampling.
//
//  2. Lightness-axis dominance during FPS. In OkLab the L axis spans 0-100
//     while a/b only swing ±30 or so in a single image, so naive
//     "pick the bin furthest from existing centroids" almost always picks
//     extreme black or extreme white from JPEG-noise outlier bins. We
//     score candidates by distance² × saliency instead — bins with very
//     low pixel count or near-zero saturation lose, salient distinctive
//     bins win.
//
// Determinism: binRGB returns sorted bins, the DBSCAN expansion visits
// points in that stable order, and cluster aggregation iterates by cluster
// ID instead of map order, so cluster IDs and rank ties resolve the same
// way every run.
const (
	dbscanMinPts   = 4 // min total pixel weight for a core point
	dbscanGridBits = 5 // 5 bits per channel → 32³ = 32 768 cells
)

type dbPoint struct {
	feat     [3]float64
	weight   uint64  // pixel count in the bin
	saliency float64 // see saliency.go (uses dbscanSaliencyPow)
	cluster  int     // 0 = unvisited, -1 = noise, ≥1 = cluster id
}

func extractDBSCAN(pixels []color.Color, k int) []color.Color {
	if len(pixels) == 0 || k <= 0 {
		return nil
	}

	points := dbscanBuildPoints(pixels)
	if len(points) == 0 {
		return nil
	}

	dbscanExpand(points)

	chosen := dbscanRankClusters(points, k)
	chosen = dbscanPadFPS(points, chosen, k)
	if len(chosen) == 0 {
		return nil
	}

	var total float64
	for _, c := range chosen {
		total += c.pixelWeight
	}
	if total == 0 {
		total = 1
	}

	out := make([]color.Color, len(chosen))
	for i, c := range chosen {
		col := color.FromOkLab(c.feat[0]/100, c.feat[1]/100, c.feat[2]/100)
		col.Freq = c.pixelWeight / total
		out[i] = col
	}
	return out
}

// dbscanBuildPoints turns binned pixels into dbPoints with OkLab×100 feats
// and saliency scores. Bins arrive sorted by RGB key from binRGB, so the
// resulting points slice is also deterministic.
func dbscanBuildPoints(pixels []color.Color) []dbPoint {
	bins := binRGB(pixels, dbscanGridBits)
	out := make([]dbPoint, len(bins))
	for i, b := range bins {
		L, a, bb := b.rgb.ToOkLab()
		out[i] = dbPoint{
			feat:     [3]float64{L * 100, a * 100, bb * 100},
			weight:   b.count,
			saliency: colorSaliency(b.rgb, b.count, dbscanSaliencyPow),
		}
	}
	return out
}

// dbscanExpand runs the standard DBSCAN expansion in-place on points,
// assigning each to a cluster ID (≥1) or marking it noise (-1). Core-point
// check uses summed pixel weights so a single high-frequency bin can
// already qualify as core.
func dbscanExpand(points []dbPoint) {
	cellOf := func(p *dbPoint) [3]int {
		return [3]int{
			int(p.feat[0] / dbScanEpsilon),
			int(p.feat[1] / dbScanEpsilon),
			int(p.feat[2] / dbScanEpsilon),
		}
	}
	grid := make(map[[3]int][]int)
	for i := range points {
		key := cellOf(&points[i])
		grid[key] = append(grid[key], i)
	}

	regionQuery := func(idx int) []int {
		c := cellOf(&points[idx])
		var out []int
		for dx := -1; dx <= 1; dx++ {
			for dy := -1; dy <= 1; dy++ {
				for dz := -1; dz <= 1; dz++ {
					ids, ok := grid[[3]int{c[0] + dx, c[1] + dy, c[2] + dz}]
					if !ok {
						continue
					}
					for _, j := range ids {
						if dbDistSq(&points[idx], &points[j]) <= dbScanEpsilon*dbScanEpsilon {
							out = append(out, j)
						}
					}
				}
			}
		}
		return out
	}

	cluster := 0
	for i := range points {
		if points[i].cluster != 0 {
			continue
		}
		neighbours := regionQuery(i)
		if dbWeightSum(points, neighbours) < dbscanMinPts {
			points[i].cluster = -1
			continue
		}
		cluster++
		points[i].cluster = cluster
		seeds := append([]int(nil), neighbours...)
		for len(seeds) > 0 {
			j := seeds[0]
			seeds = seeds[1:]
			if points[j].cluster == -1 {
				points[j].cluster = cluster
			}
			if points[j].cluster != 0 {
				continue
			}
			points[j].cluster = cluster
			jn := regionQuery(j)
			if dbWeightSum(points, jn) >= dbscanMinPts {
				seeds = append(seeds, jn...)
			}
		}
	}
}

// dbscanRankClusters aggregates cluster centroids and returns the top-K
// ranked by total saliency.
//
// Saliency-based ranking matters because the same raw-weight ranking that
// boosts dominant clusters also drowns out vibrant minor ones — a small
// saturated cluster would always rank below a large muted one if we used
// raw pixel counts.
//
// Aggregation iterates by cluster ID (not by map iteration order) so two
// runs always emit the same chosen[] sequence.
func dbscanRankClusters(points []dbPoint, k int) []dbChosen {
	maxID := 0
	for i := range points {
		if points[i].cluster > maxID {
			maxID = points[i].cluster
		}
	}
	if maxID == 0 {
		return nil
	}

	type acc struct {
		feat        [3]float64
		pixelWeight float64
		saliency    float64
	}
	clusters := make([]acc, maxID+1) // index 0 unused
	for i := range points {
		cid := points[i].cluster
		if cid <= 0 {
			continue
		}
		w := float64(points[i].weight)
		clusters[cid].feat[0] += points[i].feat[0] * w
		clusters[cid].feat[1] += points[i].feat[1] * w
		clusters[cid].feat[2] += points[i].feat[2] * w
		clusters[cid].pixelWeight += w
		clusters[cid].saliency += points[i].saliency
	}

	chosen := make([]dbChosen, 0, maxID)
	for id := 1; id <= maxID; id++ {
		c := clusters[id]
		if c.pixelWeight == 0 {
			continue
		}
		chosen = append(chosen, dbChosen{
			feat: [3]float64{
				c.feat[0] / c.pixelWeight,
				c.feat[1] / c.pixelWeight,
				c.feat[2] / c.pixelWeight,
			},
			pixelWeight: c.pixelWeight,
			saliency:    c.saliency,
		})
	}
	sort.SliceStable(chosen, func(i, j int) bool { return chosen[i].saliency > chosen[j].saliency })
	if len(chosen) > k {
		chosen = chosen[:k]
	}
	return chosen
}

// dbscanPadFPS pads chosen with farthest-point picks until len == k.
//
// The score is `nearestDistSq × saliency`, NOT pure distance. Pure-distance
// FPS in OkLab almost always picks extreme-black or extreme-white outlier
// bins, because L spans 0-100 while a/b only swing ~±30 — so the "furthest"
// point is whichever has a JPEG-noise extreme on the L axis even if its
// pixel count is 1. Multiplying by saliency suppresses those (low count
// AND low saturation → near-zero saliency) so the picks land on real
// distinctive colors.
func dbscanPadFPS(points []dbPoint, chosen []dbChosen, k int) []dbChosen {
	for len(chosen) < k {
		bestIdx := -1
		bestScore := -1.0
		for i := range points {
			d := nearestDistSq(points[i].feat, chosen)
			score := d * points[i].saliency
			if score > bestScore {
				bestScore = score
				bestIdx = i
			}
		}
		if bestIdx < 0 || bestScore <= 0 {
			break
		}
		chosen = append(chosen, dbChosen{
			feat:        points[bestIdx].feat,
			pixelWeight: float64(points[bestIdx].weight),
			saliency:    points[bestIdx].saliency,
		})
	}
	return chosen
}

// dbChosen is one already-selected centroid in the output palette — both
// DBSCAN cluster centroids and farthest-point picks land here.
//
// pixelWeight populates Freq honestly (image coverage). saliency drives
// ranking and FPS so vibrant minor colors don't lose to muted dominant
// ones.
type dbChosen struct {
	feat        [3]float64
	pixelWeight float64
	saliency    float64
}

// nearestDistSq returns the squared distance from p to its nearest member
// of chosen. Returns a large sentinel when chosen is empty so the first
// pick is unambiguous.
func nearestDistSq(p [3]float64, chosen []dbChosen) float64 {
	if len(chosen) == 0 {
		return 1e18
	}
	best := lloydDistSq(p, chosen[0].feat)
	for i := 1; i < len(chosen); i++ {
		d := lloydDistSq(p, chosen[i].feat)
		if d < best {
			best = d
		}
	}
	return best
}

func dbDistSq(a, b *dbPoint) float64 {
	return lloydDistSq(a.feat, b.feat)
}

func dbWeightSum(points []dbPoint, idx []int) uint64 {
	var s uint64
	for _, i := range idx {
		s += points[i].weight
	}
	return s
}
