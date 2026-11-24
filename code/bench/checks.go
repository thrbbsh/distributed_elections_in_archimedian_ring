package bench

import (
	"fmt"
	"math"
	"math/rand"

	"archimedean/algorithms"
	"archimedean/ring"
)

var checkTimings = []struct {
	name string
	cfg  ring.Config
}{
	{"s<=1", ring.Config{TickMin: 10, TickMax: 10}},
	{"s<=3", ring.Config{TickMin: 10, TickMax: 20, DelayMax: 10}},
	{"s<=30", ring.Config{TickMin: 10, TickMax: 100, DelayMax: 200}},
	{"s<=300", ring.Config{TickMin: 1, TickMax: 50, DelayMax: 250}},
}

type checker struct {
	rng *rand.Rand
	bad int
}

func (c *checker) verdict(ok bool, format string, args ...any) {
	if ok {
		fmt.Printf("verdict: OK, "+format+"\n\n", args...)
	} else {
		c.bad++
		fmt.Printf("verdict: DIFFERS, "+format+"\n\n", args...)
	}
}

func (c *checker) run(a ring.Algorithm, cfg ring.Config) ring.Result {
	r := ring.Run(a, cfg, rand.New(rand.NewSource(c.rng.Int63())))
	if err := r.Check(); err != nil {
		fail("%s: %v", a.Name, err)
	}
	return r
}

func (c *checker) ring(cfg ring.Config, n int, layout string, base int, init string) ring.Config {
	cfg.IDs = ring.MakeIDs(n, layout, base, c.rng)
	cfg.WakeAt, _ = ring.WakeTimes(n, init, c.rng.Int63n(10*int64(n)+1), c.rng)
	return cfg
}

func RunChecks(rng *rand.Rand) {
	c := &checker{rng: rng}
	c.bound2()
	c.fiveN()
	c.bits()
	c.timeBound()
	c.adversary()
	c.growthInL()
	c.constF()
	c.average()
	c.minTime()
	c.provedBounds()
	if c.bad == 0 {
		fmt.Println("all claims agree with the measurements")
	} else {
		fmt.Printf("%d claim(s) differ from the measurements, see above\n", c.bad)
	}
}

var layouts = []string{"random", "asc", "desc"}
var inits = []string{"all", "one", "random"}

func (c *checker) bound2() {
	fmt.Println("== 1. Section 3, bound (2), f(i) = 2^i")
	fmt.Println("claim: messages <= 2N + N (u/m) (f(l)+1) sum_i 1/f(i)  <=  2N + 3N u/m  (linear in N)")
	fmt.Println("worst of 9 rings (3 layouts x 3 wake-up modes) per row; u/m is the realised ratio of that run")
	fmt.Printf("%7s %5s %10s %10s %10s %13s %15s\n", "timing", "n", "msgs/N", "(2)/N", "2+3u/m", "msgs / (2)", "msgs/(2N+3Nu/m)")
	worst1, worst2 := 0.0, 0.0
	for _, tm := range checkTimings {
		for _, n := range []int{8, 64, 512, 2048} {
			var mx, b2, b3, r1, r2 float64
			for _, layout := range layouts {
				for _, init := range inits {
					res := c.run(algorithms.Vitanyi(algorithms.Pow2), c.ring(tm.cfg, n, layout, 1, init))
					N, s := float64(n), float64(res.U)/float64(res.M)
					tot, bound, simple := float64(res.Total()), VitanyiBound(res, algorithms.Pow2), 2*N+3*N*s
					if tot/bound > r1 {
						mx, b2, b3, r1 = tot/N, bound/N, simple/N, tot/bound
					}
					r2 = math.Max(r2, tot/simple)
				}
			}
			fmt.Printf("%7s %5d %10.2f %10.2f %10.2f %13.3f %15.3f\n", tm.name, n, mx, b2, b3, r1, r2)
			worst1, worst2 = math.Max(worst1, r1), math.Max(worst2, r2)
		}
	}
	c.verdict(worst1 <= 1 && worst2 <= 1, "largest messages/bound = %.3f for (2) and %.3f for 2N+3Nu/m", worst1, worst2)
}

func (c *checker) fiveN() {
	fmt.Println("== 2. Section 4, bound (4), f(i) = (2s)^i with s = ceil(u/m) known to the processors")
	fmt.Println("claim: messages <= 3N + N(1 + 1/f(l)) < 5N, independent of u/m")
	fmt.Printf("%7s %5s %8s %12s %12s\n", "timing", "n", "s", "max msgs/N", "f=2^i msgs/N")
	worst := 0.0
	for _, tm := range checkTimings {
		s := ring.ArchimedeanS(tm.cfg)
		for _, n := range []int{8, 512, 2048} {
			var mx, mx2 float64
			for _, layout := range layouts {
				for _, init := range inits {
					cfg := c.ring(tm.cfg, n, layout, 1, init)
					mx = math.Max(mx, float64(c.run(algorithms.Vitanyi(algorithms.Pow2S(s)), cfg).Total())/float64(n))
					mx2 = math.Max(mx2, float64(c.run(algorithms.Vitanyi(algorithms.Pow2), cfg).Total())/float64(n))
				}
			}
			fmt.Printf("%7s %5d %8d %12.3f %12.3f\n", tm.name, n, s, mx, mx2)
			worst = math.Max(worst, mx)
		}
	}
	c.verdict(worst < 5, "at most %.3f N messages", worst)
}

func (c *checker) bits() {
	fmt.Println("== 3. Section 3, bound (3) on bits, and Section 4: 2N + 3N log l bits for f(i) = (2u/m)^i")
	fmt.Println("claim (3): bits <= 2N + N (u/m) (f(l)+1) sum_i |i|/f(i), |i| = length of the name i in dyadic code")
	fmt.Println("Figure = protocol as drawn (timer := 1 on waking up: every processor sends its own name once);")
	fmt.Println("origin = M_i waits f(i) units also at its origin (timer := f(i)), as the prose of Section 3 can be read")
	fmt.Println("f(i) = 2^i, names 1..N in random order, l = 1")
	fmt.Printf("%7s %5s %14s %14s %10s %18s\n", "timing", "n", "Figure bits/N", "origin bits/N", "(3)/N", "sum_i |i| / N")
	figWorst, origWorst := 0.0, 0.0
	for _, tm := range checkTimings[:2] {
		for _, n := range []int{64, 512, 4096} {
			cfg := c.ring(tm.cfg, n, "random", 1, "all")
			fig := c.run(algorithms.Vitanyi(algorithms.Pow2), cfg)
			orig := c.run(algorithms.VitanyiOpt(algorithms.Pow2, true), cfg)
			N := float64(n)
			sum, own := 0.0, 0.0
			for _, i := range cfg.IDs {
				b := float64(ring.Bits(ring.Message{Kind: ring.Elect, Value: i}))
				sum += b / float64(algorithms.Pow2.F(i))
				own += b
			}
			bound := func(r ring.Result) float64 {
				return 2*N + N*float64(r.U)/float64(r.M)*float64(algorithms.Pow2.F(1)+1)*sum
			}
			figWorst = math.Max(figWorst, float64(fig.Bits)/bound(fig))
			origWorst = math.Max(origWorst, float64(orig.Bits)/bound(orig))
			fmt.Printf("%7s %5d %14.2f %14.2f %10.2f %18.2f\n", tm.name, n, float64(fig.Bits)/N, float64(orig.Bits)/N, bound(fig)/N, own/N)
		}
	}
	fmt.Println()
	fmt.Println("claim (4'): with f = (2s)^i, bits <= 2N + 3N log l   (origin variant, timing s<=3, n = 512)")
	fmt.Printf("%5s %10s %14s %20s\n", "l", "bits/N", "2+3*log2(l)", "2+3*|l| (dyadic)")
	s := ring.ArchimedeanS(checkTimings[1].cfg)
	viol2, viol3 := 0, 0
	for _, l := range []int{1, 2, 5, 12} {
		res := c.run(algorithms.VitanyiOpt(algorithms.Pow2S(s), true), c.ring(checkTimings[1].cfg, 512, "random", l, "all"))
		b := float64(res.Bits) / 512
		lit, dy := 2+3*math.Log2(float64(l)), 2+3*float64(ring.Bits(ring.Message{Kind: ring.Elect, Value: l}))
		if b > lit {
			viol2++
		}
		if b > dy {
			viol3++
		}
		fmt.Printf("%5d %10.2f %14.2f %20.2f\n", l, b, lit, dy)
	}
	c.verdict(false, "the protocol as drawn exceeds (3) (up to %.2f x the bound): every name is sent at least once,\n"+
		"         so for names 1..N at least sum |i| ~ N log N bits; analysis (1) misses this '+1 pass' per message.\n"+
		"         With M_i waiting also at its origin (3) holds (largest ratio %.3f). 2N + 3N log l fails in %d of 4 rows\n"+
		"         with log2 (l = 1 would allow 2N bits, M_1 alone needs N, wakeups N, sleepwell N) and in %d with the dyadic length",
		figWorst, origWorst, viol2, viol3)
}

func (c *checker) timeBound() {
	fmt.Println("== 4. Section 4, running time for f(i) = 2^i")
	fmt.Println("claim: time <= N u (2^l + 2)   (the text says \"Nu(2l+2)\"; with 2l+2 it would be wrong, see the last column)")
	fmt.Printf("%7s %5s %4s %14s %14s %14s\n", "timing", "n", "l", "time/(Nu)", "2^l + 2", "2l + 2")
	worst, litBad := 0.0, 0
	for _, tm := range checkTimings[1:3] {
		for _, l := range []int{1, 2, 4, 8, 12} {
			n := 256
			var mx float64
			for _, init := range inits {
				res := c.run(algorithms.Vitanyi(algorithms.Pow2), c.ring(tm.cfg, n, "random", l, init))
				mx = math.Max(mx, float64(res.Time)/float64(int64(n)*res.U))
			}
			b := math.Pow(2, float64(l)) + 2
			worst = math.Max(worst, mx/b)
			if mx > float64(2*l+2) {
				litBad++
			}
			fmt.Printf("%7s %5d %4d %14.2f %14.0f %14d\n", tm.name, n, l, mx, b, 2*l+2)
		}
	}
	c.verdict(worst <= 1, "time / (N u (2^l+2)) <= %.3f; the literal 2l+2 is exceeded in %d of 10 rows", worst, litBad)
}

func (c *checker) adversary() {
	fmt.Println("== 5. Section 4, worst case under adversary scheduling, f(i) = 2^i fixed")
	fmt.Println("claim: names ascending clockwise, unit delay of i = 2^(N-i+1): no overtaking, N(N+2)/2 message passes")
	fmt.Printf("%4s %10s %10s %10s %14s\n", "N", "M msgs", "N(N+1)/2", "N(N+2)/2", "all incl. 2N")
	exact := true
	for _, n := range []int{4, 8, 16, 24, 32} {
		res := c.run(algorithms.Vitanyi(algorithms.Pow2), AdversaryConfig(n))
		exact = exact && res.Count[ring.Elect] == n*(n+1)/2
		fmt.Printf("%4d %10d %10d %10d %14d\n", n, res.Count[ring.Elect], n*(n+1)/2, n*(n+2)/2, res.Total())
	}
	c.verdict(exact, "quadratic as claimed; the exact count is N(N+1)/2 election messages (M_1: N passes, M_i: N-i+1),\n"+
		"         N(N+2)/2 in the paper is N/2 more")
}

func (c *checker) growthInL() {
	fmt.Println("== 6. Section 4 and the CWI Table: dependence on the smallest name l")
	fmt.Println("claim: f = i^a (a > 1): O(N u l/m) messages; f = 2^i: O(N u/m), independent of l; f = (2u/m)^i: Theta(N)")
	fmt.Println("timing s<=3, n = 512, names l..l+511, worst of 3 layouts")
	fmt.Println("n/a: f(l) is above the simulator's cap 2^40, where all messages would travel equally fast (not the paper's f)")
	fmt.Printf("%5s %12s %12s %14s %14s\n", "l", "i^2 msgs/N", "i^2: (2)/N", "2^i msgs/N", "(2s)^i msgs/N")
	tm := checkTimings[1].cfg
	s := ring.ArchimedeanS(tm)
	fs := []algorithms.DelayFunc{algorithms.Pow2, algorithms.Pow2S(s)}
	lo, hi := []float64{math.Inf(1), math.Inf(1)}, []float64{0, 0}
	var first, last float64
	ok := true
	for _, l := range []int{1, 4, 8, 12, 64, 256} {
		var sq, b float64
		ex := []float64{0, 0}
		for _, layout := range layouts {
			cfg := c.ring(tm, 512, layout, l, "all")
			r := c.run(algorithms.Vitanyi(algorithms.Square), cfg)
			sq, b = math.Max(sq, float64(r.Total())/512), math.Max(b, VitanyiBound(r, algorithms.Square)/512)
			ok = ok && float64(r.Total()) <= VitanyiBound(r, algorithms.Square)
			for k, f := range fs {
				if f.F(l) < fCap {
					ex[k] = math.Max(ex[k], float64(c.run(algorithms.Vitanyi(f), cfg).Total())/512)
				}
			}
		}
		if l == 1 {
			first = sq
		}
		last = sq
		cell := make([]string, 2)
		for k, f := range fs {
			if f.F(l) >= fCap {
				cell[k] = "n/a"
				continue
			}
			cell[k] = fmt.Sprintf("%.2f", ex[k])
			lo[k], hi[k] = math.Min(lo[k], ex[k]), math.Max(hi[k], ex[k])
		}
		fmt.Printf("%5d %12.2f %12.1f %14s %14s\n", l, sq, b, cell[0], cell[1])
	}
	c.verdict(ok && hi[0] < 1.3*lo[0] && hi[1] < 1.3*lo[1],
		"for i^2 the messages grow with l (x%.1f from l=1 to l=256) and stay below (2), which grows linearly in l;\n"+
			"         for 2^i (%.2f..%.2f) and (2s)^i (%.2f..%.2f) they do not depend on l", last/first, lo[0], hi[0], lo[1], hi[1])
}

func (c *checker) constF() {
	fmt.Println("== 7. CWI Table, f = constant (no use of time): O(N^2) worst case, as Le Lann / Chang-Roberts")
	fmt.Printf("%6s %14s %14s %14s\n", "n", "asc msgs/N", "random msgs/N", "2^i asc msgs/N")
	var prev, growth float64
	for _, n := range []int{64, 256, 1024} {
		asc := c.run(algorithms.Vitanyi(algorithms.Const), c.ring(checkTimings[0].cfg, n, "asc", 1, "all"))
		rnd := c.run(algorithms.Vitanyi(algorithms.Const), c.ring(checkTimings[0].cfg, n, "random", 1, "all"))
		p2 := c.run(algorithms.Vitanyi(algorithms.Pow2), c.ring(checkTimings[0].cfg, n, "asc", 1, "all"))
		a := float64(asc.Total()) / float64(n)
		if prev > 0 {
			growth = a / prev
		}
		prev = a
		fmt.Printf("%6d %14.2f %14.2f %14.2f\n", n, a, float64(rnd.Total())/float64(n), float64(p2.Total())/float64(n))
	}
	fmt.Println("(synchronous ring s = 1, names ascending clockwise: the worst layout when the minimum wins)")
	c.verdict(growth > 3.5, "with f = 1, quadrupling n multiplies messages/N by %.2f: quadratic", growth)
}

func (c *checker) average() {
	fmt.Println("== 8. Section 4, average case (5), and the asynchronous algorithms it is compared with")
	fmt.Println("claim: f = 2^i, l = 1: O(N) on average; Chang-Roberts: N H_N election messages on average [CR];")
	fmt.Println("       Peterson / DKR: at most 2N log N + N election messages; the asynchronous lower bound is Omega(N log N)")
	fmt.Printf("%6s %12s %12s %16s %14s %18s\n", "n", "V msgs/N", "CR elect/N", "CR / (N H_N)", "P elect/N", "P / (2N log N + N)")
	tm := checkTimings[1].cfg
	okCR, okP := true, true
	var vFirst, vLast float64
	for _, n := range []int{64, 256, 1024, 4096} {
		trials := 20
		if n == 4096 {
			trials = 5
		}
		var v, cr, p, pmax float64
		for k := 0; k < trials; k++ {
			cfg := c.ring(tm, n, "random", 1, "all")
			v += float64(c.run(algorithms.Vitanyi(algorithms.Pow2), cfg).Total())
			cr += float64(c.run(algorithms.ChangRoberts, cfg).Election())
			pe := float64(c.run(algorithms.Peterson, cfg).Election())
			p += pe
			pmax = math.Max(pmax, pe)
		}
		N, t := float64(n), float64(trials)
		h := 0.0
		for i := 1; i <= n; i++ {
			h += 1 / float64(i)
		}
		crRatio, pRatio := cr/t/(N*h), pmax/(2*N*math.Log2(N)+N)
		okCR = okCR && math.Abs(crRatio-1) < 0.05
		okP = okP && pRatio <= 1
		if n == 64 {
			vFirst = v / t / N
		}
		vLast = v / t / N
		fmt.Printf("%6d %12.2f %12.2f %16.3f %14.2f %18.3f\n", n, v/t/N, cr/t/N, crRatio, p/t/N, pRatio)
	}
	c.verdict(okCR && okP && vLast < 1.2*vFirst,
		"Vitanyi stays at about %.1f messages per processor while Chang-Roberts grows like H_N", vLast)
}

func (c *checker) minTime() {
	fmt.Println("== 9. Section 4, minimal time variant f(i, j) = floor(2^(j-i))")
	fmt.Println("claim: time about 3 walk times (wakeup, M_l, sleepwell each circle once), whatever l is;")
	fmt.Println("       but O(N^2) messages in the worst case and O(N log N) on average (\"a reversion to [CR]\")")
	fmt.Printf("%6s %5s %16s %16s %14s %14s\n", "n", "l", "time/(3Nu)", "2^i: time/(3Nu)", "asc msgs/N", "random msgs/N")
	tm := checkTimings[1].cfg
	worst := 0.0
	var ascPrev, ascGrowth float64
	for _, n := range []int{64, 256, 1024} {
		for _, l := range []int{1, 10} {
			rnd := c.run(algorithms.Vitanyi(algorithms.MinTime), c.ring(tm, n, "random", l, "all"))
			asc := c.run(algorithms.Vitanyi(algorithms.MinTime), c.ring(tm, n, "asc", l, "all"))
			p2 := c.run(algorithms.Vitanyi(algorithms.Pow2), c.ring(tm, n, "random", l, "all"))
			tr := float64(rnd.Time) / float64(3*int64(n)*rnd.U)
			worst = math.Max(worst, math.Max(tr, float64(asc.Time)/float64(3*int64(n)*asc.U)))
			a := float64(asc.Total()) / float64(n)
			if l == 1 {
				if ascPrev > 0 {
					ascGrowth = a / ascPrev
				}
				ascPrev = a
			}
			fmt.Printf("%6d %5d %16.3f %16.3f %14.2f %14.2f\n", n, l, tr, float64(p2.Time)/float64(3*int64(n)*p2.U), a, float64(rnd.Total())/float64(n))
		}
	}
	c.verdict(worst <= 1 && ascGrowth > 3.5, "time <= %.3f * 3Nu for every l, ascending ring quadratic (x%.2f per x4 in n)", worst, ascGrowth)
}

func (c *checker) provedBounds() {
	fmt.Println("== 10. Bounds proved in this work (they count the first hop of every message and the wake-up delay)")
	fmt.Println("claim M: messages <= 4N - 1 + N (u/m) (f(l)+2) sum_{i != l} 1/f(i);  f = 2^i: < 4N + 2N u/m;  f = (2s)^i: < 6N")
	fmt.Println("claim T: time from the first wake-up <= (N-1) dmax + N (f(l)+1) u + N u  <  N u (f(l)+3)")
	fmt.Printf("%7s %8s %6s %14s %14s\n", "timing", "f", "runs", "max msgs/(M)", "max time/(T)")
	worstM, worstT := 0.0, 0.0
	for _, tm := range checkTimings {
		s := ring.ArchimedeanS(tm.cfg)
		for _, f := range []algorithms.DelayFunc{algorithms.Pow2, algorithms.Pow2S(s), algorithms.Square} {
			var rm, rt float64
			runs := 0
			for _, n := range []int{8, 64, 512, 2048} {
				for _, layout := range layouts {
					for _, init := range inits {
						cfg := c.ring(tm.cfg, n, layout, 1+c.rng.Intn(3), init)
						res := c.run(algorithms.Vitanyi(f), cfg)
						runs++
						N, l := float64(n), ring.MinID(cfg.IDs)
						sum := 0.0
						for _, i := range cfg.IDs {
							if i != l {
								sum += 1 / float64(f.F(i))
							}
						}
						fl := float64(f.F(l))
						u, m := float64(res.U), float64(res.M)
						bm := 4*N - 1 + N*u/m*(fl+2)*sum
						t0 := int64(-1)
						for _, w := range cfg.WakeAt {
							if w >= 0 && (t0 < 0 || w < t0) {
								t0 = w
							}
						}
						bt := (N-1)*float64(cfg.DelayMax) + N*(fl+1)*u + N*u
						rm = math.Max(rm, float64(res.Total())/bm)
						rt = math.Max(rt, float64(res.Time-t0)/bt)
					}
				}
			}
			fmt.Printf("%7s %8s %6d %14.3f %14.3f\n", tm.name, f.Name, runs, rm, rt)
			worstM, worstT = math.Max(worstM, rm), math.Max(worstT, rt)
		}
	}
	c.verdict(worstM <= 1 && worstT <= 1, "largest ratios %.3f (messages) and %.3f (time)", worstM, worstT)
}
