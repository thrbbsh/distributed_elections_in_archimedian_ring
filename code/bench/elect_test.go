package bench

import (
	"math"
	"math/rand"
	"testing"

	"archimedean/algorithms"
	"archimedean/ring"
)

func init() { ring.ShowLogs = false }

var timings = []struct {
	name string
	cfg  ring.Config
}{
	{"sync", ring.Config{TickMin: 10, TickMax: 10, DelayMin: 0, DelayMax: 0}},
	{"mild", ring.Config{TickMin: 10, TickMax: 20, DelayMin: 0, DelayMax: 10}},
	{"wild", ring.Config{TickMin: 1, TickMax: 100, DelayMin: 0, DelayMax: 500}},
}

func allAlgorithms() []ring.Algorithm {
	return []ring.Algorithm{algorithms.ChangRoberts, algorithms.Peterson, algorithms.Vitanyi(algorithms.Pow2), algorithms.Vitanyi(algorithms.Pow2S(3)), algorithms.Vitanyi(algorithms.Square), algorithms.Vitanyi(algorithms.Const),
		algorithms.Vitanyi(algorithms.MinTime), algorithms.VitanyiOpt(algorithms.Pow2, true)}
}

func newRing(t *testing.T, cfg ring.Config, ids []int, init string, spread int64, rng *rand.Rand) ring.Config {
	t.Helper()
	cfg.IDs = ids
	var err error
	if cfg.WakeAt, err = ring.WakeTimes(len(ids), init, spread, rng); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestElectsCorrectLeader(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for _, algo := range allAlgorithms() {
		for _, tm := range timings {
			for n := 1; n <= 40; n++ {
				for _, layout := range []string{"random", "asc", "desc"} {
					for _, init := range []string{"all", "one", "random"} {
						cfg := newRing(t, tm.cfg, ring.MakeIDs(n, layout, 1+rng.Intn(5), rng), init, rng.Int63n(1000), rng)
						if err := ring.Run(algo, cfg, rng).Check(); err != nil {
							t.Fatalf("%s, %s, n=%d, %s, init %s, ids %v: %v", algo.Name, tm.name, n, layout, init, cfg.IDs, err)
						}
					}
				}
			}
		}
	}
}

func TestVitanyiCorrectForAnyTiming(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	for trial := 0; trial < 300; trial++ {
		n := 1 + rng.Intn(60)
		cfg := ring.Config{TickMin: 1, TickMax: 1 + rng.Int63n(100000), DelayMax: rng.Int63n(1000000)}
		ids := rng.Perm(10 * n)[:n]
		for i := range ids {
			ids[i]++
		}
		cfg = newRing(t, cfg, ids, "random", rng.Int63n(1000000), rng)
		for _, f := range []algorithms.DelayFunc{algorithms.Pow2, algorithms.Square, algorithms.Const} {
			if err := ring.Run(algorithms.Vitanyi(f), cfg, rng).Check(); err != nil {
				t.Fatalf("f=%s, ids %v: %v", f.Name, ids, err)
			}
		}
	}
}

func TestVitanyiBound(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	for _, tm := range timings {
		for trial := 0; trial < 40; trial++ {
			n := 2 + rng.Intn(500)
			cfg := newRing(t, tm.cfg, ring.MakeIDs(n, "random", 1, rng), []string{"all", "one", "random"}[trial%3], rng.Int63n(5000), rng)
			res := ring.Run(algorithms.Vitanyi(algorithms.Pow2), cfg, rng)
			N, s := float64(n), float64(res.U)/float64(res.M)
			if b := VitanyiBound(res, algorithms.Pow2); float64(res.Total()) > b {
				t.Fatalf("%s n=%d: %d messages > bound (2) = %.1f", tm.name, n, res.Total(), b)
			}
			if b := 2*N + 3*N*s; float64(res.Total()) > b {
				t.Fatalf("%s n=%d: %d messages > 2N + 3N u/m = %.1f", tm.name, n, res.Total(), b)
			}
		}
	}
}

func TestVitanyiFiveN(t *testing.T) {
	rng := rand.New(rand.NewSource(4))
	for _, tm := range timings {
		s := ring.ArchimedeanS(tm.cfg)
		for trial := 0; trial < 40; trial++ {
			n := 1 + rng.Intn(500)
			cfg := newRing(t, tm.cfg, ring.MakeIDs(n, "random", 1, rng), "random", rng.Int63n(5000), rng)
			if res := ring.Run(algorithms.Vitanyi(algorithms.Pow2S(s)), cfg, rng); res.Total() >= 5*n {
				t.Fatalf("%s (s=%d) n=%d: %d messages >= 5N", tm.name, s, n, res.Total())
			}
		}
	}
}

func TestVitanyiAdversary(t *testing.T) {
	for n := 2; n <= 30; n++ {
		res := ring.Run(algorithms.Vitanyi(algorithms.Pow2), AdversaryConfig(n), rand.New(rand.NewSource(5)))
		if err := res.Check(); err != nil {
			t.Fatal(err)
		}
		if want := n * (n + 1) / 2; res.Count[ring.Elect] != want || res.Total() != want+2*n {
			t.Fatalf("n=%d: %d election messages, %d in total; want %d and %d", n, res.Count[ring.Elect], res.Total(), want, want+2*n)
		}
	}
}

func TestVitanyiTimeGrowsWithSmallestName(t *testing.T) {
	cfg := ring.Config{TickMin: 10, TickMax: 20, DelayMax: 10}
	timeFor := func(base int) int64 {
		c := cfg
		c.IDs = ring.MakeIDs(50, "random", base, rand.New(rand.NewSource(6)))
		res := ring.Run(algorithms.Vitanyi(algorithms.Pow2), c, rand.New(rand.NewSource(7)))
		if err := res.Check(); err != nil {
			t.Fatal(err)
		}
		return res.Time
	}
	t1, t10 := timeFor(1), timeFor(10)
	if t10 < 100*t1 {
		t.Fatalf("time with l = 10 is %d, with l = 1 it is %d: expected a factor of about 2^9", t10, t1)
	}
}

func TestChangRobertsExtremes(t *testing.T) {
	for n := 1; n <= 60; n++ {
		for layout, want := range map[string]int{"desc": n * (n + 1) / 2, "asc": 2*n - 1} {
			cfg := ring.Config{TickMin: 10, TickMax: 20, DelayMax: 10, IDs: ring.MakeIDs(n, layout, 1, nil)}
			if res := ring.Run(algorithms.ChangRoberts, cfg, rand.New(rand.NewSource(8))); res.Count[ring.Elect] != want {
				t.Fatalf("n=%d %s: %d messages, want %d", n, layout, res.Count[ring.Elect], want)
			}
		}
	}
}

func TestPetersonBound(t *testing.T) {
	rng := rand.New(rand.NewSource(9))
	for n := 1; n <= 1024; n = n*3/2 + 1 {
		for _, layout := range []string{"random", "asc", "desc"} {
			cfg := ring.Config{TickMin: 10, TickMax: 20, DelayMax: 10, IDs: ring.MakeIDs(n, layout, 1, rng)}
			res := ring.Run(algorithms.Peterson, cfg, rng)
			if limit := 2*n*int(math.Floor(math.Log2(float64(n)))) + n; res.Election() > limit {
				t.Fatalf("n=%d %s: %d messages > %d", n, layout, res.Election(), limit)
			}
		}
	}
}

func TestVitanyiBeatsAsynchronous(t *testing.T) {
	rng := rand.New(rand.NewSource(10))
	cfg := ring.Config{TickMin: 10, TickMax: 20, DelayMax: 10, IDs: ring.MakeIDs(2000, "random", 1, rng)}
	v := ring.Run(algorithms.Vitanyi(algorithms.Pow2), cfg, rand.New(rand.NewSource(11))).Total()
	cr := ring.Run(algorithms.ChangRoberts, cfg, rand.New(rand.NewSource(11))).Total()
	p := ring.Run(algorithms.Peterson, cfg, rand.New(rand.NewSource(11))).Total()
	if v >= cr || v >= p {
		t.Fatalf("Vitanyi %d, Chang-Roberts %d, Peterson %d messages", v, cr, p)
	}
}

func TestDeterministic(t *testing.T) {
	cfg := ring.Config{TickMin: 5, TickMax: 50, DelayMax: 40, IDs: ring.MakeIDs(300, "random", 1, rand.New(rand.NewSource(12)))}
	a := ring.Run(algorithms.Vitanyi(algorithms.Pow2), cfg, rand.New(rand.NewSource(13)))
	b := ring.Run(algorithms.Vitanyi(algorithms.Pow2), cfg, rand.New(rand.NewSource(13)))
	if a.Total() != b.Total() || a.Time != b.Time {
		t.Fatalf("same seed, different runs")
	}
}

func TestExampleRing(t *testing.T) {
	ids := []int{5, 2, 6, 3, 1, 4, 8, 7}
	for _, algo := range []ring.Algorithm{algorithms.ChangRoberts, algorithms.Peterson, algorithms.Vitanyi(algorithms.Pow2)} {
		cfg := ring.Config{TickMin: 10, TickMax: 20, DelayMax: 10, IDs: ids}
		res := ring.Run(algo, cfg, rand.New(rand.NewSource(14)))
		if err := res.Check(); err != nil {
			t.Fatalf("%s: %v", algo.Name, err)
		}
		if want := map[bool]int{true: 4, false: 6}[algo.Name[0] == 'V']; algo.OwnerWins && res.LeaderPos != want {
			t.Fatalf("%s: leader P%d, want P%d", algo.Name, res.LeaderPos, want)
		}
	}
}

func TestVitanyiBits(t *testing.T) {
	rng := rand.New(rand.NewSource(15))
	cfg := ring.Config{TickMin: 10, TickMax: 10, IDs: ring.MakeIDs(4096, "random", 1, rng)}
	fig := ring.Run(algorithms.Vitanyi(algorithms.Pow2), cfg, rand.New(rand.NewSource(16)))
	orig := ring.Run(algorithms.VitanyiOpt(algorithms.Pow2, true), cfg, rand.New(rand.NewSource(16)))
	own := 0
	for _, i := range cfg.IDs {
		own += ring.Bits(ring.Message{Kind: ring.Elect, Value: i})
	}
	if fig.Bits < own {
		t.Fatalf("Figure: %d bits, but the own names alone take %d", fig.Bits, own)
	}
	if n := len(cfg.IDs); orig.Bits > 4*n {
		t.Fatalf("wait at origin: %d bits > 4N", orig.Bits)
	}
}
