package bench

import (
	"fmt"
	"math/rand"
	"os"

	"archimedean/algorithms"
	"archimedean/ring"
)

func VitanyiBound(res ring.Result, f algorithms.DelayFunc) float64 {
	n := float64(len(res.IDs))
	l := ring.MinID(res.IDs)
	sum := 0.0
	for _, i := range res.IDs {
		sum += 1 / float64(f.F(i))
	}
	return 2*n + n*float64(res.U)/float64(res.M)*float64(f.F(l)+1)*sum
}

func AdversaryConfig(n int) ring.Config {
	cfg := ring.Config{DelayMin: 0, DelayMax: 0}
	for i := 1; i <= n; i++ {
		cfg.IDs = append(cfg.IDs, i)
		cfg.Periods = append(cfg.Periods, int64(1)<<(n-i+1))
	}
	return cfg
}

func RunBench(maxN, trials int, timing ring.Config, rng *rand.Rand) {
	cfg0 := timing
	s := ring.ArchimedeanS(cfg0)
	fmt.Printf("All messages (wakeups and announcement included).\n")
	fmt.Printf("clock periods in [%d, %d], link delays in [%d, %d]: u = %d, m = %d, s = ceil(u/m) = %d\n",
		timing.TickMin, timing.TickMax, timing.DelayMin, timing.DelayMax, timing.TickMax+timing.DelayMax, timing.TickMin, s)
	fmt.Printf("names 1..n; random = average over %d random rings; asc / desc = sorted along the ring.\n", trials)
	fmt.Printf("CR = Chang-Roberts, P = Peterson, V = Vitanyi with f = 2^i, V2s = Vitanyi with f = (2s)^i.\n\n")
	fmt.Printf("%6s %7s %9s %9s %8s %8s %6s %6s %6s %6s %10s %10s\n",
		"n", "layout", "CR", "P", "V", "V2s", "CR/n", "P/n", "V/n", "V2s/n", "bound(2)", "V time/u")
	for n := 8; n <= maxN; n *= 2 {
		for _, layout := range []string{"random", "asc", "desc"} {
			t := trials
			if layout != "random" {
				t = 1
			}
			var cr, p, v, v2, bound, vt float64
			for k := 0; k < t; k++ {
				cfg := cfg0
				cfg.IDs = ring.MakeIDs(n, layout, 1, rng)
				seed := rng.Int63()
				run := func(a ring.Algorithm) ring.Result {
					r := ring.Run(a, cfg, rand.New(rand.NewSource(seed)))
					if err := r.Check(); err != nil {
						fail("n=%d %s %s: %v", n, layout, a.Name, err)
					}
					return r
				}
				cr += float64(run(algorithms.ChangRoberts).Total())
				p += float64(run(algorithms.Peterson).Total())
				rv := run(algorithms.Vitanyi(algorithms.Pow2))
				v += float64(rv.Total())
				bound += VitanyiBound(rv, algorithms.Pow2)
				vt += float64(rv.Time) / float64(rv.U)
				v2 += float64(run(algorithms.Vitanyi(algorithms.Pow2S(s))).Total())
			}
			ft, fn := float64(t), float64(n)
			fmt.Printf("%6d %7s %9.0f %9.0f %8.0f %8.0f %6.2f %6.2f %6.2f %6.2f %10.0f %10.1f\n",
				n, layout, cr/ft, p/ft, v/ft, v2/ft, cr/ft/fn, p/ft/fn, v/ft/fn, v2/ft/fn, bound/ft, vt/ft)
		}
	}
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

const fCap = int64(1) << 40
