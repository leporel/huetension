package extract

import (
	"sort"

	"github.com/leporel/huetension/internal/color"
)

// Weighted k-means with saliency proxy + over-clustering.
//
// Goal: surface small but distinctive color regions that plain Lloyd at low
// K tends to drown in their larger neighbours. The two compounding reasons
// plain Lloyd misses them:
//
//  1. Frequency dominance — a minor region contributes little to the
//     clustering loss, so Lloyd happily merges it into a neighbour.
//  2. Centroid scarcity — at small K there are simply not enough centroids
//     for each natural color region to claim its own seed.
//
// We address both with a three-stage pipeline:
//
//  1. Over-cluster with K' = K × wkOverClusterFactor (capped). This gives
//     Lloyd enough centroids that distinctive minor regions can claim a
//     seed instead of being absorbed into a larger neighbour.
//  2. Run weighted Lloyd via the shared lloyd.go primitives, with saliency
//     as the per-point weight so vibrant pixels pull centroids toward
//     themselves more strongly than count alone would justify.
//  3. Merge centroids that ended up perceptually close (ΔE OkLab below
//     wkMeansMergeEpsilon), then rank survivors by saliency and take top-K.
//
// Output Freq is pixel-count fraction so consumers can still see "what
// share of the image this color covers" — saliency drives ranking, not the
// reported coverage.
//
// Determinism: the binning step (binRGB) returns sorted entries, the PRNG
// is seeded from the input pixels (seedRNGFromPixels), and Lloyd visits
// points in a stable order. Two runs on the same image produce identical
// palettes across processes.

const (
	wkBinBitsRGB = 5
)

// wkPoint is one binned observation with both pixelCount (honest coverage,
// drives output Freq) and saliency (drives Lloyd weighting and ranking).
type wkPoint struct {
	feat       [3]float64
	pixelCount float64
	saliency   float64
}

func extractWeightedKMeans(pixels []color.Color, k int) []color.Color {
	if len(pixels) == 0 || k <= 0 {
		return nil
	}

	points := wkBuildPoints(pixels)
	if len(points) == 0 {
		return nil
	}
	if k > len(points) {
		k = len(points)
	}

	overK := pickOverK(k, len(points), wkMaxOverK)
	rng := seedRNGFromPixels(pixels)
	centers, assignments := runLloyd(wkToLloydPoints(points), overK, rng)

	clusters := wkBuildClusters(points, centers, assignments, overK)
	clusters = wkMergeCloseClusters(clusters, wkMeansMergeEpsilon)
	clusters = wkTopBySaliency(clusters, k)
	return wkClustersToColors(clusters)
}

// wkBuildPoints turns binned pixels into wkPoints in OkLab×100 feature
// space. Saliency uses wkSaliencyPow for an aggressive vibrance bias.
func wkBuildPoints(pixels []color.Color) []wkPoint {
	bins := binRGB(pixels, wkBinBitsRGB)
	out := make([]wkPoint, len(bins))
	for i, b := range bins {
		L, a, bb := b.rgb.ToOkLab()
		out[i] = wkPoint{
			feat:       [3]float64{L * 100, a * 100, bb * 100},
			pixelCount: float64(b.count),
			saliency:   colorSaliency(b.rgb, b.count, wkSaliencyPow),
		}
	}
	return out
}

// wkToLloydPoints projects wkPoints into the generic Lloyd point format,
// using saliency as the centroid-update weight.
func wkToLloydPoints(points []wkPoint) []lloydPoint {
	out := make([]lloydPoint, len(points))
	for i, p := range points {
		out[i] = lloydPoint{feat: p.feat, weight: p.saliency}
	}
	return out
}

// wkCluster aggregates everything we need about an over-clustered group:
// pixel-weighted OkLab centroid, total pixel coverage, total saliency.
type wkCluster struct {
	feat        [3]float64
	pixelWeight float64
	saliency    float64
}

// wkBuildClusters folds Lloyd's per-point assignments into one wkCluster
// per centroid. Centroid is the pixel-weighted mean (where the color
// actually is), independent of the saliency-weighted centroid Lloyd used.
func wkBuildClusters(points []wkPoint, centers [][3]float64, assignments []int, k int) []wkCluster {
	clusters := make([]wkCluster, k)
	for j := range k {
		clusters[j].feat = centers[j]
	}
	pixelSums := make([][3]float64, k)
	for i, p := range points {
		j := assignments[i]
		pixelSums[j][0] += p.feat[0] * p.pixelCount
		pixelSums[j][1] += p.feat[1] * p.pixelCount
		pixelSums[j][2] += p.feat[2] * p.pixelCount
		clusters[j].pixelWeight += p.pixelCount
		clusters[j].saliency += p.saliency
	}
	for j := range k {
		if clusters[j].pixelWeight > 0 {
			clusters[j].feat = [3]float64{
				pixelSums[j][0] / clusters[j].pixelWeight,
				pixelSums[j][1] / clusters[j].pixelWeight,
				pixelSums[j][2] / clusters[j].pixelWeight,
			}
		}
	}
	out := clusters[:0]
	for _, c := range clusters {
		if c.pixelWeight > 0 {
			out = append(out, c)
		}
	}
	return out
}

// wkMergeCloseClusters collapses cluster pairs whose ΔE in OkLab is below
// eps. Each merge is pixel-weighted (so the centroid stays where the
// colors actually are) and saliencies sum (so the merged cluster's rank
// reflects the combined importance of its members).
func wkMergeCloseClusters(clusters []wkCluster, eps float64) []wkCluster {
	for {
		bestI, bestJ := -1, -1
		bestD := eps * eps
		for i := 0; i < len(clusters); i++ {
			for j := i + 1; j < len(clusters); j++ {
				d := lloydDistSq(clusters[i].feat, clusters[j].feat)
				if d < bestD {
					bestD = d
					bestI, bestJ = i, j
				}
			}
		}
		if bestI < 0 {
			return clusters
		}
		clusters[bestI] = wkMergeClusters(clusters[bestI], clusters[bestJ])
		clusters = append(clusters[:bestJ], clusters[bestJ+1:]...)
	}
}

func wkMergeClusters(a, b wkCluster) wkCluster {
	total := a.pixelWeight + b.pixelWeight
	if total == 0 {
		return a
	}
	return wkCluster{
		feat: [3]float64{
			(a.feat[0]*a.pixelWeight + b.feat[0]*b.pixelWeight) / total,
			(a.feat[1]*a.pixelWeight + b.feat[1]*b.pixelWeight) / total,
			(a.feat[2]*a.pixelWeight + b.feat[2]*b.pixelWeight) / total,
		},
		pixelWeight: total,
		saliency:    a.saliency + b.saliency,
	}
}

// wkTopBySaliency keeps the K most salient clusters. Ranking by saliency
// rather than pixel weight is the whole point of this method — it's how
// minor saturated colors survive into the final palette.
func wkTopBySaliency(clusters []wkCluster, k int) []wkCluster {
	sort.SliceStable(clusters, func(i, j int) bool {
		return clusters[i].saliency > clusters[j].saliency
	})
	if len(clusters) > k {
		clusters = clusters[:k]
	}
	return clusters
}

// wkClustersToColors converts ranked clusters into the final palette. Freq
// is computed from pixelWeight only (over the surviving K), so consumers
// see "what share of the image this color represents" honestly.
func wkClustersToColors(clusters []wkCluster) []color.Color {
	if len(clusters) == 0 {
		return nil
	}
	var totalPixels float64
	for _, c := range clusters {
		totalPixels += c.pixelWeight
	}
	if totalPixels == 0 {
		totalPixels = 1
	}
	out := make([]color.Color, len(clusters))
	for i, c := range clusters {
		col := color.FromOkLab(c.feat[0]/100, c.feat[1]/100, c.feat[2]/100)
		col.Freq = c.pixelWeight / totalPixels
		out[i] = col
	}
	return out
}
