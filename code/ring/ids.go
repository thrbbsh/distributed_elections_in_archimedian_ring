package ring

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
)

func MinID(ids []int) int {
	m := ids[0]
	for _, id := range ids {
		if id < m {
			m = id
		}
	}
	return m
}

func MaxID(ids []int) int {
	m := ids[0]
	for _, id := range ids {
		if id > m {
			m = id
		}
	}
	return m
}

func ArchimedeanS(cfg Config) int64 {
	lo, hi := cfg.TickMin, cfg.TickMax
	if cfg.Periods != nil {
		lo, hi = cfg.Periods[0], cfg.Periods[0]
		for _, p := range cfg.Periods {
			lo, hi = min(lo, p), max(hi, p)
		}
	}
	u := hi + cfg.DelayMax
	return (u + lo - 1) / lo
}

func MakeIDs(n int, layout string, base int, rng *rand.Rand) []int {
	ids := make([]int, n)
	for i := range ids {
		ids[i] = base + i
	}
	switch layout {
	case "random":
		rng.Shuffle(n, func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
	case "desc":
		for i := range ids {
			ids[i] = base + n - 1 - i
		}
	}
	return ids
}

func ReadIDs(idsFlag, file string, n int, layout string, base int, rng *rand.Rand) ([]int, error) {
	text := idsFlag
	if file != "" {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		text = string(data)
	}
	if text == "" {
		if n < 1 || base < 1 {
			return nil, fmt.Errorf("-n and -base must be at least 1")
		}
		if layout != "random" && layout != "asc" && layout != "desc" {
			return nil, fmt.Errorf("unknown layout %q", layout)
		}
		return MakeIDs(n, layout, base, rng), nil
	}
	var ids []int
	seen := map[int]bool{}
	sc := bufio.NewScanner(strings.NewReader(strings.ReplaceAll(text, ",", " ")))
	sc.Split(bufio.ScanWords)
	for sc.Scan() {
		id, err := strconv.Atoi(sc.Text())
		if err != nil || id < 1 {
			return nil, fmt.Errorf("bad name %q: names must be positive integers", sc.Text())
		}
		if seen[id] {
			return nil, fmt.Errorf("name %d appears twice: names must be distinct", id)
		}
		seen[id] = true
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("no names given")
	}
	return ids, nil
}

func WakeTimes(n int, mode string, spread int64, rng *rand.Rand) ([]int64, error) {
	at := make([]int64, n)
	when := func() int64 {
		if spread <= 0 {
			return 0
		}
		return rng.Int63n(spread + 1)
	}
	switch mode {
	case "all":
		for i := range at {
			at[i] = when()
		}
	case "one":
		for i := range at {
			at[i] = -1
		}
		at[rng.Intn(n)] = when()
	case "random":
		any := false
		for i := range at {
			at[i] = -1
			if rng.Intn(2) == 0 {
				at[i], any = when(), true
			}
		}
		if !any {
			at[rng.Intn(n)] = when()
		}
	default:
		return nil, fmt.Errorf("unknown -init %q, use all, one or random", mode)
	}
	return at, nil
}
