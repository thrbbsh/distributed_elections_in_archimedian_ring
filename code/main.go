package main

import (
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"strings"
	"time"

	"archimedean/algorithms"
	"archimedean/bench"
	"archimedean/ring"
)

func main() {
	n := flag.Int("n", 8, "number of processors (when no -ids/-file is given)")
	layout := flag.String("layout", "random", "names for -n: random | asc | desc (in ring order P0, P1, ...)")
	base := flag.Int("base", 1, "names for -n are base, base+1, ..., base+n-1 (the smallest name l = base)")
	idsFlag := flag.String("ids", "", "comma-separated names in ring order, e.g. 5,2,6,3,1,4,8,7")
	file := flag.String("file", "", "file with the names in ring order")
	algoName := flag.String("algo", "vitanyi", "vitanyi | cr | peterson | all")
	fName := flag.String("f", "2", "Vitanyi delay function: 2 = 2^i, 2s = (2s)^i, sq = i^2, 1 = constant, mt = 2^(j-i) (minimal time)")
	origin := flag.Bool("origin", false, "Vitanyi: M_i waits f(i) units also at its origin (timer := f(i) instead of 1 on waking up)")
	tickMin := flag.Int64("tickmin", 10, "shortest clock period m (absolute time units)")
	tickMax := flag.Int64("tickmax", 20, "longest clock period")
	delayMin := flag.Int64("delaymin", 0, "shortest link delay")
	delayMax := flag.Int64("delaymax", 10, "longest link delay")
	initMode := flag.String("init", "all", "who wakes up spontaneously: all | one | random")
	spread := flag.Int64("spread", 0, "spontaneous wake-ups happen at random times in [0, spread]")
	adversary := flag.Bool("adversary", false, "Section 4 worst case: names 1..n ascending, period of name i = 2^(n-i+1), no delays")
	seed := flag.Int64("seed", 0, "random seed (0 = from the clock); printed, so a run can be repeated")
	flag.DurationVar(&ring.Delay, "delay", 0, "pause before every logged step, e.g. 200ms")
	quiet := flag.Bool("quiet", false, "print only the summary")
	runBench := flag.Bool("bench", false, "print the message table for n = 8, 16, ..., -maxn")
	check := flag.Bool("check", false, "compare the claims of the paper with measurements")
	table := flag.Bool("table", false, "compare all algorithms for n = 8, 16, ..., -maxn in the synchronous model")
	maxN := flag.Int("maxn", 2048, "largest n for -bench and -table")
	trials := flag.Int("trials", 10, "random rings per n for -bench and -table")
	flag.Parse()

	if *seed == 0 {
		*seed = time.Now().UnixNano() % 1_000_000
	}
	rng := rand.New(rand.NewSource(*seed))
	if *tickMin < 1 || *tickMax < *tickMin || *delayMin < 0 || *delayMax < *delayMin {
		fail("need 1 <= tickmin <= tickmax and 0 <= delaymin <= delaymax")
	}
	timing := ring.Config{TickMin: *tickMin, TickMax: *tickMax, DelayMin: *delayMin, DelayMax: *delayMax}

	switch {
	case *runBench:
		ring.ShowLogs = false
		bench.RunBench(*maxN, *trials, timing, rng)
		return
	case *check:
		ring.ShowLogs = false
		fmt.Printf("seed %d\n\n", *seed)
		bench.RunChecks(rng)
		return
	case *table:
		ring.ShowLogs = false
		bench.RunTable(*maxN, *trials, rng)
		return
	}

	var cfg ring.Config
	if *adversary {
		if *n > 40 {
			fail("-adversary needs n <= 40 (the clock periods are 2^n)")
		}
		cfg = bench.AdversaryConfig(*n)
		*fName = "2"
	} else {
		ids, err := ring.ReadIDs(*idsFlag, *file, *n, *layout, *base, rng)
		if err != nil {
			fail("%v", err)
		}
		cfg = timing
		cfg.IDs = ids
		cfg.WakeAt, err = ring.WakeTimes(len(ids), *initMode, *spread, rng)
		if err != nil {
			fail("%v", err)
		}
	}

	var algos []ring.Algorithm
	switch strings.ToLower(*algoName) {
	case "vitanyi", "v":
		algos = []ring.Algorithm{algorithms.VitanyiOpt(delayFunc(*fName, cfg), *origin)}
	case "cr", "chang-roberts":
		algos = []ring.Algorithm{algorithms.ChangRoberts}
	case "peterson", "p":
		algos = []ring.Algorithm{algorithms.Peterson}
	case "all":
		algos = []ring.Algorithm{algorithms.ChangRoberts, algorithms.Peterson, algorithms.VitanyiOpt(delayFunc(*fName, cfg), *origin)}
		*quiet = true
	default:
		fail("unknown algorithm %q", *algoName)
	}
	ring.ShowLogs = !*quiet

	for _, algo := range algos {
		fmt.Printf("%s on a ring of %d processors, seed %d\n", algo.Name, len(cfg.IDs), *seed)
		fmt.Printf("messages go P0 -> P1 -> ... -> P%d -> P0\n", len(cfg.IDs)-1)
		if len(cfg.IDs) <= 64 {
			fmt.Printf("names: %v\n\n", cfg.IDs)
		}
		res := ring.Run(algo, cfg, rand.New(rand.NewSource(*seed)))
		printSummary(res, algo)
		if err := res.Check(); err != nil {
			fail("WRONG RESULT: %v", err)
		}
		fmt.Println()
	}
}

func delayFunc(name string, cfg ring.Config) algorithms.DelayFunc {
	switch name {
	case "2":
		return algorithms.Pow2
	case "2s":
		return algorithms.Pow2S(ring.ArchimedeanS(cfg))
	case "sq":
		return algorithms.Square
	case "1":
		return algorithms.Const
	case "mt":
		return algorithms.MinTime
	}
	fail("unknown -f %q, use 2, 2s, sq, 1 or mt", name)
	return algorithms.DelayFunc{}
}

func delayFuncOf(a ring.Algorithm) algorithms.DelayFunc {
	name := strings.TrimSuffix(strings.TrimPrefix(a.Name, "Vitanyi f="), " (wait at origin)")
	for _, f := range []algorithms.DelayFunc{algorithms.Pow2, algorithms.Square, algorithms.Const, algorithms.MinTime} {
		if f.Name == name {
			return f
		}
	}
	var s int64
	fmt.Sscanf(name, "(2*%d)^i", &s)
	return algorithms.Pow2S(s)
}

func printSummary(res ring.Result, algo ring.Algorithm) {
	n := float64(len(res.IDs))
	fmt.Printf("================ summary: %s ================\n", res.Algo)
	fmt.Printf("n                          %d\n", len(res.IDs))
	fmt.Printf("u, m, u/m                  %d, %d, %.2f\n", res.U, res.M, float64(res.U)/float64(res.M))
	if res.LeaderPos >= 0 {
		fmt.Printf("leader                     P%d (name %d), elected with value %d (must be %d)\n",
			res.LeaderPos, res.IDs[res.LeaderPos], res.LeaderValue, res.Winner)
	}
	for _, k := range []ring.Kind{ring.Wakeup, ring.Elect, ring.P1, ring.P2, ring.AnnounceLeader} {
		if c := res.Count[k]; c > 0 {
			fmt.Printf("%-26s %d\n", k.String()+" messages", c)
		}
	}
	fmt.Printf("all messages               %d   (%.2f per processor)\n", res.Total(), float64(res.Total())/n)
	if n > 1 {
		fmt.Printf("  per n log2 n             %.3f\n", float64(res.Total())/(n*math.Log2(n)))
	}
	if f := delayFuncOf(algo); strings.HasPrefix(res.Algo, "Vitanyi") && f.F != nil {
		fmt.Printf("  bound (2) of the paper   %.1f\n", bench.VitanyiBound(res, f))
	}
	fmt.Printf("bits (names in dyadic)     %d   (%.2f per processor)\n", res.Bits, float64(res.Bits)/n)
	fmt.Printf("time of the last event     %d   (%.1f longest ticks)\n", res.Time, float64(res.Time)/float64(res.U))
	fmt.Printf("ticks with an action       %d\n", res.Ticks)
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
