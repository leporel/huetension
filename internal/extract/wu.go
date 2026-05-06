package extract

import "github.com/leporel/huetension/internal/color"

// Wu quantization (Xiaolin Wu, "Efficient Statistical Computations for
// Optimal Color Quantization", Graphics Gems vol. II, 1991).
//
// Pipeline:
//
//  1. Bin pixels into a 32×32×32 RGB histogram (5-bit per channel).
//  2. Build cumulative weight, R, G, B, and squared-norm moment tables so
//     that any axis-aligned sub-cube's totals can be computed in O(1) via
//     inclusion-exclusion of its eight corners.
//  3. Repeatedly split the cube whose split offers the largest variance
//     reduction along its best axis, until ≤ K cubes remain.
//  4. Each surviving cube produces one palette color (its weighted mean).
//
// Deterministic; runs in well under 100 ms on a 256×256 image.

const (
	wuQuant      = 33  // 32 + 1 — index 0 is reserved for inclusion-exclusion margins.
	wuBits       = 5   // bits kept per channel after histogram binning.
	wuMaxClusters = 256 // upper bound on K before we cap.
)

type wuBox struct {
	r0, r1 int
	g0, g1 int
	b0, b1 int
	vol    int
}

type wuQuantizer struct {
	wt [wuQuant][wuQuant][wuQuant]int64   // weight (= pixel count)
	mr [wuQuant][wuQuant][wuQuant]int64   // sum of R
	mg [wuQuant][wuQuant][wuQuant]int64   // sum of G
	mb [wuQuant][wuQuant][wuQuant]int64   // sum of B
	m2 [wuQuant][wuQuant][wuQuant]float64 // sum of R²+G²+B²
}

func extractWu(pixels []color.Color, k int) []color.Color {
	if len(pixels) == 0 {
		return nil
	}
	if k <= 0 {
		k = 1
	}
	if k > wuMaxClusters {
		k = wuMaxClusters
	}

	q := &wuQuantizer{}
	q.hist3d(pixels)
	q.m3d()

	boxes := make([]wuBox, k)
	boxes[0] = wuBox{r1: 32, g1: 32, b1: 32}
	vv := make([]float64, k) // variance reduction per box, to pick the next cut

	nb := 1
	next := 0
	for i := 1; i < k; i++ {
		if q.cut(&boxes[next], &boxes[i]) {
			vv[next] = q.variance(&boxes[next])
			vv[i] = q.variance(&boxes[i])
		} else {
			vv[next] = 0
			i--
		}
		// Pick the box with the most variance to cut next.
		next = 0
		var maxV float64
		for j := 0; j <= i; j++ {
			if vv[j] > maxV {
				maxV = vv[j]
				next = j
			}
		}
		nb = i + 1
		if maxV <= 0 {
			break
		}
	}

	// Build palette from each box's weighted average.
	out := make([]color.Color, 0, nb)
	var totalW int64
	weights := make([]int64, nb)
	for i := range nb {
		weights[i] = q.vol(&boxes[i], &q.wt)
		totalW += weights[i]
	}
	for i := range nb {
		w := weights[i]
		if w == 0 {
			continue
		}
		rs := q.vol(&boxes[i], &q.mr)
		gs := q.vol(&boxes[i], &q.mg)
		bs := q.vol(&boxes[i], &q.mb)
		c := color.New(uint8(rs/w), uint8(gs/w), uint8(bs/w))
		if totalW > 0 {
			c.Freq = float64(w) / float64(totalW)
		}
		out = append(out, c)
	}
	return out
}

// hist3d bins pixels into a 32×32×32 histogram. The +1 offset keeps index 0
// available for the inclusion-exclusion baseline used by vol().
func (q *wuQuantizer) hist3d(pixels []color.Color) {
	for _, c := range pixels {
		ir := int(c.R>>(8-wuBits)) + 1
		ig := int(c.G>>(8-wuBits)) + 1
		ib := int(c.B>>(8-wuBits)) + 1
		q.wt[ir][ig][ib]++
		q.mr[ir][ig][ib] += int64(c.R)
		q.mg[ir][ig][ib] += int64(c.G)
		q.mb[ir][ig][ib] += int64(c.B)
		q.m2[ir][ig][ib] += float64(int(c.R)*int(c.R) + int(c.G)*int(c.G) + int(c.B)*int(c.B))
	}
}

// m3d turns the per-cell histograms into 3D cumulative moments so any sub-cube
// total is an 8-corner inclusion-exclusion away (see vol).
func (q *wuQuantizer) m3d() {
	var area, areaR, areaG, areaB [wuQuant]int64
	var area2 [wuQuant]float64

	for r := 1; r <= 32; r++ {
		for i := 0; i <= 32; i++ {
			area[i], areaR[i], areaG[i], areaB[i], area2[i] = 0, 0, 0, 0, 0
		}
		for g := 1; g <= 32; g++ {
			var line, lineR, lineG, lineB int64
			var line2 float64
			for b := 1; b <= 32; b++ {
				line += q.wt[r][g][b]
				lineR += q.mr[r][g][b]
				lineG += q.mg[r][g][b]
				lineB += q.mb[r][g][b]
				line2 += q.m2[r][g][b]

				area[b] += line
				areaR[b] += lineR
				areaG[b] += lineG
				areaB[b] += lineB
				area2[b] += line2

				q.wt[r][g][b] = q.wt[r-1][g][b] + area[b]
				q.mr[r][g][b] = q.mr[r-1][g][b] + areaR[b]
				q.mg[r][g][b] = q.mg[r-1][g][b] + areaG[b]
				q.mb[r][g][b] = q.mb[r-1][g][b] + areaB[b]
				q.m2[r][g][b] = q.m2[r-1][g][b] + area2[b]
			}
		}
	}
}

// vol returns the integer-moment sum over the box defined by b. The eight
// corners are combined per the standard 3D inclusion-exclusion formula.
func (q *wuQuantizer) vol(b *wuBox, mom *[wuQuant][wuQuant][wuQuant]int64) int64 {
	return mom[b.r1][b.g1][b.b1] -
		mom[b.r1][b.g1][b.b0] -
		mom[b.r1][b.g0][b.b1] +
		mom[b.r1][b.g0][b.b0] -
		mom[b.r0][b.g1][b.b1] +
		mom[b.r0][b.g1][b.b0] +
		mom[b.r0][b.g0][b.b1] -
		mom[b.r0][b.g0][b.b0]
}

func (q *wuQuantizer) volF(b *wuBox, mom *[wuQuant][wuQuant][wuQuant]float64) float64 {
	return mom[b.r1][b.g1][b.b1] -
		mom[b.r1][b.g1][b.b0] -
		mom[b.r1][b.g0][b.b1] +
		mom[b.r1][b.g0][b.b0] -
		mom[b.r0][b.g1][b.b1] +
		mom[b.r0][b.g1][b.b0] +
		mom[b.r0][b.g0][b.b1] -
		mom[b.r0][b.g0][b.b0]
}

// variance is the residual variance of b after subtracting its mean colour.
func (q *wuQuantizer) variance(b *wuBox) float64 {
	dr := float64(q.vol(b, &q.mr))
	dg := float64(q.vol(b, &q.mg))
	db := float64(q.vol(b, &q.mb))
	xx := q.volF(b, &q.m2)
	w := float64(q.vol(b, &q.wt))
	if w == 0 {
		return 0
	}
	return xx - (dr*dr+dg*dg+db*db)/w
}

// bottom returns the moment of the lower face of b in direction dir — used as
// a base by maximize so each candidate cut only needs the small "Top(pos)"
// delta on top.
func (q *wuQuantizer) bottom(b *wuBox, dir int, mom *[wuQuant][wuQuant][wuQuant]int64) int64 {
	switch dir {
	case 0: // R axis
		return -mom[b.r0][b.g1][b.b1] + mom[b.r0][b.g1][b.b0] +
			mom[b.r0][b.g0][b.b1] - mom[b.r0][b.g0][b.b0]
	case 1: // G axis
		return -mom[b.r1][b.g0][b.b1] + mom[b.r1][b.g0][b.b0] +
			mom[b.r0][b.g0][b.b1] - mom[b.r0][b.g0][b.b0]
	case 2: // B axis
		return -mom[b.r1][b.g1][b.b0] + mom[b.r1][b.g0][b.b0] +
			mom[b.r0][b.g1][b.b0] - mom[b.r0][b.g0][b.b0]
	}
	return 0
}

// top returns the moment of the upper "slice" up to position pos in direction dir.
func (q *wuQuantizer) top(b *wuBox, dir, pos int, mom *[wuQuant][wuQuant][wuQuant]int64) int64 {
	switch dir {
	case 0:
		return mom[pos][b.g1][b.b1] - mom[pos][b.g1][b.b0] -
			mom[pos][b.g0][b.b1] + mom[pos][b.g0][b.b0]
	case 1:
		return mom[b.r1][pos][b.b1] - mom[b.r1][pos][b.b0] -
			mom[b.r0][pos][b.b1] + mom[b.r0][pos][b.b0]
	case 2:
		return mom[b.r1][b.g1][pos] - mom[b.r1][b.g0][pos] -
			mom[b.r0][b.g1][pos] + mom[b.r0][b.g0][pos]
	}
	return 0
}

// maximize returns the largest variance-reduction value reachable by any cut
// of b along dir between (first, last), and the position that achieves it
// (or -1 if no valid cut exists).
func (q *wuQuantizer) maximize(b *wuBox, dir, first, last int, wholeR, wholeG, wholeB, wholeW int64) (float64, int) {
	baseR := q.bottom(b, dir, &q.mr)
	baseG := q.bottom(b, dir, &q.mg)
	baseB := q.bottom(b, dir, &q.mb)
	baseW := q.bottom(b, dir, &q.wt)

	var maxV float64
	bestPos := -1
	for i := first; i < last; i++ {
		hr := baseR + q.top(b, dir, i, &q.mr)
		hg := baseG + q.top(b, dir, i, &q.mg)
		hb := baseB + q.top(b, dir, i, &q.mb)
		hw := baseW + q.top(b, dir, i, &q.wt)
		if hw == 0 {
			continue
		}
		temp := (float64(hr)*float64(hr) + float64(hg)*float64(hg) + float64(hb)*float64(hb)) / float64(hw)

		hr = wholeR - hr
		hg = wholeG - hg
		hb = wholeB - hb
		hw = wholeW - hw
		if hw == 0 {
			continue
		}
		temp += (float64(hr)*float64(hr) + float64(hg)*float64(hg) + float64(hb)*float64(hb)) / float64(hw)

		if temp > maxV {
			maxV = temp
			bestPos = i
		}
	}
	return maxV, bestPos
}

// cut splits set1 into set1 (the lower half along the chosen axis) and set2
// (the upper half). Returns false if no valid cut can be found.
func (q *wuQuantizer) cut(set1, set2 *wuBox) bool {
	wholeW := q.vol(set1, &q.wt)
	wholeR := q.vol(set1, &q.mr)
	wholeG := q.vol(set1, &q.mg)
	wholeB := q.vol(set1, &q.mb)

	maxR, cutR := q.maximize(set1, 0, set1.r0+1, set1.r1, wholeR, wholeG, wholeB, wholeW)
	maxG, cutG := q.maximize(set1, 1, set1.g0+1, set1.g1, wholeR, wholeG, wholeB, wholeW)
	maxB, cutB := q.maximize(set1, 2, set1.b0+1, set1.b1, wholeR, wholeG, wholeB, wholeW)

	var dir int
	switch {
	case maxR >= maxG && maxR >= maxB:
		dir = 0
		if cutR < 0 {
			return false
		}
	case maxG >= maxR && maxG >= maxB:
		dir = 1
		if cutG < 0 {
			return false
		}
	default:
		dir = 2
		if cutB < 0 {
			return false
		}
	}

	set2.r1, set2.g1, set2.b1 = set1.r1, set1.g1, set1.b1
	switch dir {
	case 0:
		set2.r0 = cutR
		set1.r1 = cutR
		set2.g0, set2.b0 = set1.g0, set1.b0
	case 1:
		set2.g0 = cutG
		set1.g1 = cutG
		set2.r0, set2.b0 = set1.r0, set1.b0
	case 2:
		set2.b0 = cutB
		set1.b1 = cutB
		set2.r0, set2.g0 = set1.r0, set1.g0
	}
	set1.vol = (set1.r1 - set1.r0) * (set1.g1 - set1.g0) * (set1.b1 - set1.b0)
	set2.vol = (set2.r1 - set2.r0) * (set2.g1 - set2.g0) * (set2.b1 - set2.b0)
	return true
}
