package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const SourcePath = "/proc/spl/kstat/zfs/arcstats"

type Stat struct {
	Hits, Misses     uint64
	L2Hits, L2Misses uint64
}

func GetStats() (Stat, error) {
	f, err := os.Open(SourcePath)
	if err != nil {
		return Stat{}, err
	}
	defer f.Close()
	var s Stat
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 3 {
			continue
		}
		switch fields[0] {
		case "hits":
			s.Hits, err = strconv.ParseUint(fields[2], 10, 64)
		case "misses":
			s.Misses, err = strconv.ParseUint(fields[2], 10, 64)
		case "l2_hits":
			s.L2Hits, err = strconv.ParseUint(fields[2], 10, 64)
		case "l2_misses":
			s.L2Misses, err = strconv.ParseUint(fields[2], 10, 64)
		}
		if err != nil {
			return Stat{}, err
		}
	}
	return s, nil
}

func main() {
	intervalP := flag.Duration("i", time.Second, "interval")
	flag.Parse()
	interval := intervalP.Seconds()

	last, err := GetStats()
	if err != nil {
		panic(err)
	}

	ticker := time.NewTicker(*intervalP)
	defer ticker.Stop()
	for range ticker.C {
		s, err := GetStats()
		if err != nil {
			panic(err)
		}

		hits := s.Hits - last.Hits
		misses := s.Misses - last.Misses
		l2hits := s.L2Hits - last.L2Hits
		l2misses := s.L2Misses - last.L2Misses

		hitrate := float64(hits) / float64(interval)
		missrate := float64(misses) / float64(interval)
		l2hitrate := float64(l2hits) / float64(interval)
		l2missrate := float64(l2misses) / float64(interval)
		reqrate := float64(hits+misses) / float64(interval)
		hitratio, l2hitratio := 0.0, 0.0
		if hits+misses > 0 {
			hitratio = float64(hits) / float64(hits+misses) * 100.0
		}
		if l2hits+l2misses > 0 {
			l2hitratio = float64(l2hits) / float64(l2hits+l2misses) * 100.0
		}

		fmt.Printf("%.1f req/s, ARC %.1f/%.1f (%.1f%%), L2ARC %.1f/%.1f (%.1f%%)\n",
			reqrate, hitrate, missrate, hitratio, l2hitrate, l2missrate, l2hitratio)

		last = s
	}
}
