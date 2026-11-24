package bench

import (
	"fmt"
	"math"
	"math/rand"

	"archimedean/algorithms"
	"archimedean/ring"
)

func RunTable(maxN, trials int, rng *rand.Rand) {
	sync := ring.Config{TickMin: 1, TickMax: 1, DelayMin: 1, DelayMax: 1}
	s := ring.ArchimedeanS(sync)
	algos := []ring.Algorithm{algorithms.ChangRoberts, algorithms.Peterson, algorithms.Vitanyi(algorithms.Square), algorithms.Vitanyi(algorithms.Pow2), algorithms.Vitanyi(algorithms.Pow2S(s))}
	fmt.Printf("Synchronous model: tick = 1, link = 1 time unit, all wake up at t = 0; names 1..n.\n")
	fmt.Printf("avg = average over %d random rings; worst = largest of the random rings and the rings with names\n", trials)
	fmt.Printf("ascending / descending along the ring. All messages counted (wakeups and announcement included).\n")
	fmt.Printf("Time = time units until the last processor knows the leader (= ticks).\n\n")
	fmt.Printf("%6s | %-20s | %9s | %8s | %10s | %8s | %9s | %10s\n",
		"n", "algorithm", "messages", "msgs/n", "worst/n", "bits/n", "time", "worst time")
	for n := 8; n <= maxN; n *= 2 {
		for _, a := range algos {
			var msgs, bits, tm, worstM, worstT float64
			run := func(ids []int) ring.Result {
				cfg := sync
				cfg.IDs = ids
				r := ring.Run(a, cfg, rand.New(rand.NewSource(rng.Int63())))
				if err := r.Check(); err != nil {
					fail("%s: %v", a.Name, err)
				}
				worstM = math.Max(worstM, float64(r.Total()))
				worstT = math.Max(worstT, float64(r.Time))
				return r
			}
			for k := 0; k < trials; k++ {
				r := run(ring.MakeIDs(n, "random", 1, rng))
				msgs += float64(r.Total())
				bits += float64(r.Bits)
				tm += float64(r.Time)
			}
			run(ring.MakeIDs(n, "asc", 1, rng))
			run(ring.MakeIDs(n, "desc", 1, rng))
			t, N := float64(trials), float64(n)
			fmt.Printf("%6d | %-20s | %9.0f | %8.2f | %10.2f | %8.2f | %9.0f | %10.0f\n",
				n, a.Name, msgs/t, msgs/t/N, worstM/N, bits/t/N, tm/t, worstT)
		}
	}
}
