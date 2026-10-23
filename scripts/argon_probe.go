//go:build ignore

// Command argon_probe measures the machine and prints a recommended limit and
// burst for the password_hashing limiter, for a few targets. Prod only.
//
// The argon2id parameters follow the password handling section of
// https://words.filippo.io/right-answers/.
//
// Usage (from repo root):
//
//	go run scripts/argon_probe.go
package main

import (
	"crypto/rand"
	"fmt"
	"math"
	"os"
	"path"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/crypto/argon2"
)

const (
	// argon2id parameters used by the password handlers.
	argonTime    = 2
	argonMemory  = 64 * 1024 // KiB, Go counts KiB
	argonThreads = 1
	argonKeyLen  = 32
	argonSaltLen = 16

	// hashRounds is how many rounds the timings run. The first round warms up
	// and is not recorded.
	hashRounds = 5

	// cgroupNoLimit is the "unlimited" value cgroup v1 rounds down to the page
	// size. Anything at or above it means no limit.
	cgroupNoLimit = 1 << 62
)

// targetPercents is the share of RAM and CPU each recommendation is made for.
var targetPercents = []int{10, 30, 50, 70, 90}

// cpuCount returns the number of CPUs the process can use.
func cpuCount() int {
	return runtime.NumCPU()
}

// gomaxprocs returns the number of threads the Go runtime runs at once.
func gomaxprocs() int {
	return runtime.GOMAXPROCS(0)
}

// totalRAM returns the total memory of the machine, in bytes.
func totalRAM() uint64 {
	var info syscall.Sysinfo_t
	err := syscall.Sysinfo(&info)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "read total memory: %v\n", err)
		os.Exit(1)
	}
	return uint64(info.Totalram) * uint64(info.Unit)
}

// availableRAM returns the memory available for new work, in bytes, read from
// MemAvailable.
func availableRAM() uint64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "read meminfo: %v\n", err)
		os.Exit(1)
	}

	for line := range strings.SplitSeq(string(data), "\n") {
		if !strings.HasPrefix(line, "MemAvailable:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		kib, parseErr := strconv.ParseUint(fields[1], 10, 64)
		if parseErr != nil {
			_, _ = fmt.Fprintf(os.Stderr, "parse MemAvailable: %v\n", parseErr)
			os.Exit(1)
		}
		return kib * 1024
	}

	_, _ = fmt.Fprintf(os.Stderr, "MemAvailable not found in /proc/meminfo\n")
	os.Exit(1)
	return 0
}

// cgroupMemoryMax returns the cgroup memory limit in bytes, if one is set.
func cgroupMemoryMax() (uint64, bool) {
	limitData, err := os.ReadFile("/sys/fs/cgroup/memory/memory.limit_in_bytes")
	if err == nil {
		limit, parseErr := strconv.ParseUint(strings.TrimSpace(string(limitData)), 10, 64)
		if parseErr == nil && limit > 0 && limit < cgroupNoLimit {
			return limit, true
		}
		return 0, false
	}

	var minLimit uint64
	subPath := cgroupV2SubPath()
	for {
		maxData, readErr := os.ReadFile(path.Join("/sys/fs/cgroup", subPath, "memory.max"))
		if readErr == nil {
			value := strings.TrimSpace(string(maxData))
			if value != "max" {
				limit, parseErr := strconv.ParseUint(value, 10, 64)
				if parseErr == nil && limit > 0 && (minLimit == 0 || limit < minLimit) {
					minLimit = limit
				}
			}
		}
		if subPath == "/" || subPath == "." {
			break
		}
		subPath = path.Dir(subPath)
	}
	if minLimit == 0 {
		return 0, false
	}
	return minLimit, true
}

// cgroupV2SubPath returns the cgroup v2 path of the current process.
func cgroupV2SubPath() string {
	data, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return "/"
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if !strings.HasPrefix(line, "0::") {
			continue
		}
		return strings.TrimPrefix(line, "0::")
	}
	return "/"
}

// cgroupCPUQuota returns the cgroup CPU quota in cores, if one is set.
func cgroupCPUQuota() (float64, bool) {
	quotaData, err := os.ReadFile("/sys/fs/cgroup/cpu/cpu.cfs_quota_us")
	if err == nil {
		periodData, periodErr := os.ReadFile("/sys/fs/cgroup/cpu/cpu.cfs_period_us")
		if periodErr == nil {
			quota, quotaErr := strconv.ParseInt(strings.TrimSpace(string(quotaData)), 10, 64)
			period, periodParseErr := strconv.ParseInt(strings.TrimSpace(string(periodData)), 10, 64)
			if quotaErr == nil && periodParseErr == nil && quota > 0 && period > 0 {
				return float64(quota) / float64(period), true
			}
		}
		return 0, false
	}

	var minQuota float64
	subPath := cgroupV2SubPath()
	for {
		maxData, readErr := os.ReadFile(path.Join("/sys/fs/cgroup", subPath, "cpu.max"))
		if readErr == nil {
			quota, parseErr := parseCPUMax(strings.TrimSpace(string(maxData)))
			if parseErr == nil && quota > 0 && (minQuota == 0 || quota < minQuota) {
				minQuota = quota
			}
		}
		if subPath == "/" || subPath == "." {
			break
		}
		subPath = path.Dir(subPath)
	}
	if minQuota == 0 {
		return 0, false
	}
	return minQuota, true
}

// parseCPUMax returns the CPU quota in cores from the contents of a cgroup v2
// cpu.max file ("quota period", or "max period" for no limit).
func parseCPUMax(value string) (float64, error) {
	bounds := strings.Fields(value)
	if len(bounds) == 0 {
		return 0, fmt.Errorf("empty cpu.max")
	}

	quota, err := strconv.ParseFloat(bounds[0], 64)
	if err != nil {
		return 0, fmt.Errorf("parse cpu.max quota %q: %w", bounds[0], err)
	}

	period := 100000.0
	if len(bounds) > 1 {
		period, err = strconv.ParseFloat(bounds[1], 64)
		if err != nil {
			return 0, fmt.Errorf("parse cpu.max period %q: %w", bounds[1], err)
		}
	}
	return quota / period, nil
}

// hashTiming returns the middle time of one hash alone, and the middle number
// of hashes per second with one hash per CPU. Both come from the same rounds,
// so the two always agree.
func hashTiming() (time.Duration, float64) {
	password := []byte("restinpieces-argon-probe")
	workers := cpuCount()

	salts := make([][]byte, workers)
	for i := range salts {
		salts[i] = make([]byte, argonSaltLen)
		_, err := rand.Read(salts[i])
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "read salt: %v\n", err)
			os.Exit(1)
		}
	}

	solo := make([]time.Duration, 0, hashRounds-1)
	batch := make([]time.Duration, 0, hashRounds-1)
	for round := range hashRounds {
		start := time.Now()
		argon2.IDKey(password, salts[0], argonTime, argonMemory, argonThreads, argonKeyLen)
		soloTime := time.Since(start)

		start = time.Now()
		var wg sync.WaitGroup
		for i := range workers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				argon2.IDKey(password, salts[i], argonTime, argonMemory, argonThreads, argonKeyLen)
			}()
		}
		wg.Wait()
		batchTime := time.Since(start)

		// The first round warms up; its time is thrown away.
		if round == 0 {
			continue
		}
		solo = append(solo, soloTime)
		batch = append(batch, batchTime)
	}

	slices.Sort(solo)
	slices.Sort(batch)
	soloMiddle := solo[len(solo)/2]
	batchMiddle := batch[len(batch)/2]

	return soloMiddle, float64(workers) / batchMiddle.Seconds()
}

// hashMemory returns the memory one hash uses, in bytes.
func hashMemory() uint64 {
	return argonMemory * 1024
}

// twoSignificant rounds value to two significant figures.
func twoSignificant(value float64) float64 {
	if value == 0 {
		return 0
	}
	magnitude := math.Pow(10, math.Floor(math.Log10(math.Abs(value)))-1)
	return math.Round(value/magnitude) * magnitude
}

// targetLimit returns the limit for a target that uses percent of the CPU.
func targetLimit(percent int, hashesPerSecond float64) time.Duration {
	seconds := 1 / (float64(percent) / 100 * hashesPerSecond)
	milliseconds := twoSignificant(seconds * 1000)
	return time.Duration(milliseconds) * time.Millisecond
}

// targetBurst returns the burst for a target that may use percent of the
// available memory. The core count caps the result: a hash occupies one core
// for its whole run, so a burst above the core count only multiplies the time
// each request waits and does not raise throughput. Available memory, not
// total memory, is the ceiling, because the rest belongs to the running
// application. The result is at least one, because a burst of zero would
// refuse every login while leaving the hash itself just as large.
func targetBurst(percent int, available uint64) int {
	byRAM := int(available * uint64(percent) / 100 / hashMemory())
	byCPU := cpuCount()

	burst := byRAM
	if byCPU < burst {
		burst = byCPU
	}
	if burst < 1 {
		burst = 1
	}

	return burst
}

func main() {
	ramTotal := totalRAM()
	ramAvailable := availableRAM()

	solo, busy := hashTiming()

	cgroupMemory, hasCgroupMemory := cgroupMemoryMax()
	cgroupMemoryText := "none"
	if hasCgroupMemory {
		cgroupMemoryText = fmt.Sprintf("%.1f GiB", float64(cgroupMemory)/(1<<30))
	}

	cgroupCPU, hasCgroupCPU := cgroupCPUQuota()
	cgroupCPUText := "none"
	if hasCgroupCPU {
		cgroupCPUText = fmt.Sprintf("%.1f cores", cgroupCPU)
	}

	_, _ = fmt.Printf("machine\n")
	_, _ = fmt.Printf("  cpu cores              %d\n", cpuCount())
	_, _ = fmt.Printf("  GOMAXPROCS             %d\n", gomaxprocs())
	_, _ = fmt.Printf("  RAM total              %.1f GiB\n", float64(ramTotal)/(1<<30))
	_, _ = fmt.Printf("  RAM available          %.1f GiB\n", float64(ramAvailable)/(1<<30))
	_, _ = fmt.Printf("  cgroup memory.max      %s\n", cgroupMemoryText)
	_, _ = fmt.Printf("  cgroup cpu.max         %s\n", cgroupCPUText)

	_, _ = fmt.Printf("\n")
	_, _ = fmt.Printf("argon2id (m=%d, t=%d, p=%d)\n", argonMemory, argonTime, argonThreads)
	_, _ = fmt.Printf("  time per hash          %.0f ms\n", solo.Seconds()*1000)
	_, _ = fmt.Printf("  memory per hash        %d MiB\n", hashMemory()/(1<<20))
	_, _ = fmt.Printf("  one at a time          %.1f hashes/s\n", 1/solo.Seconds())
	_, _ = fmt.Printf("  all cores busy         ~%.0f hashes/s  (knee at %d running at once)\n", busy, cpuCount())

	_, _ = fmt.Printf("\n")
	_, _ = fmt.Printf("recommendation\n")
	for _, percent := range targetPercents {
		_, _ = fmt.Printf("  at most %d%% of RAM and CPU    limit=%s  burst=%d\n", percent, targetLimit(percent, busy), targetBurst(percent, ramAvailable))
	}
}
