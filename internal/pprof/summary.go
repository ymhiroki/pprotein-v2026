package pprof

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/google/pprof/profile"
)

type (
	// Summary lists the functions with the largest values in a profile, like
	// `go tool pprof -top` (sorted by flat) and `go tool pprof -top -cum` (sorted by cum).
	// It uses the profile's default sample type, as go tool pprof does.
	Summary struct {
		SampleType string        // "cpu" for CPU profiles, "time" for fgprof
		Duration   time.Duration // 0 if the profile does not record it
		Total      string        // sum of all samples, formatted
		Functions  int           // number of distinct functions in the profile
		Shown      int           // number of rows in each table
		ByFlat     string        // table sorted by flat
		ByCum      string        // table sorted by cum
	}
)

// Summarize reads the profile at path and returns the top n functions.
func Summarize(path string, n int) (*Summary, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open profile: %w", err)
	}
	defer f.Close()

	p, err := profile.Parse(f)
	if err != nil {
		return nil, fmt.Errorf("failed to parse profile: %w", err)
	}
	idx, err := p.SampleIndexByName("")
	if err != nil || idx < 0 {
		return nil, fmt.Errorf("profile has no sample types")
	}
	unit := p.SampleType[idx].Unit

	// flat counts a sample for the innermost function only; cum counts it once for
	// every function on the stack.
	var total int64
	flat := map[string]int64{}
	cum := map[string]int64{}
	for _, s := range p.Sample {
		if idx >= len(s.Value) {
			continue
		}
		v := s.Value[idx]
		total += v

		seen := map[string]bool{}
		for i, loc := range s.Location {
			for j, name := range functionNames(loc) {
				if i == 0 && j == 0 {
					flat[name] += v
				}
				if !seen[name] {
					seen[name] = true
					cum[name] += v
				}
			}
		}
	}

	names := make([]string, 0, len(cum))
	for name := range cum {
		names = append(names, name)
	}
	shown := min(n, len(names))

	return &Summary{
		SampleType: p.SampleType[idx].Type,
		Duration:   time.Duration(p.DurationNanos),
		Total:      formatValue(total, unit),
		Functions:  len(names),
		Shown:      shown,
		ByFlat:     table(sortedBy(names, flat, cum), shown, flat, cum, total, unit),
		ByCum:      table(sortedBy(names, cum, flat), shown, flat, cum, total, unit),
	}, nil
}

// functionNames returns the functions of a location, innermost first. A location has
// more than one function when calls were inlined.
func functionNames(loc *profile.Location) []string {
	if len(loc.Line) == 0 {
		return []string{fmt.Sprintf("%#x", loc.Address)}
	}
	names := make([]string, len(loc.Line))
	for i, line := range loc.Line {
		names[i] = "?"
		if line.Function != nil {
			names[i] = line.Function.Name
		}
	}
	return names
}

// sortedBy returns names sorted by primary, then secondary, both descending.
func sortedBy(names []string, primary, secondary map[string]int64) []string {
	sorted := append([]string(nil), names...)
	sort.Slice(sorted, func(a, b int) bool {
		na, nb := sorted[a], sorted[b]
		if primary[na] != primary[nb] {
			return primary[na] > primary[nb]
		}
		if secondary[na] != secondary[nb] {
			return secondary[na] > secondary[nb]
		}
		return na < nb
	})
	return sorted
}

func table(names []string, n int, flat, cum map[string]int64, total int64, unit string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%10s %7s %7s %10s %7s  %s\n", "flat", "flat%", "sum%", "cum", "cum%", "function")
	var sum int64
	for _, name := range names[:n] {
		sum += flat[name]
		fmt.Fprintf(&b, "%10s %6.2f%% %6.2f%% %10s %6.2f%%  %s\n",
			formatValue(flat[name], unit), percent(flat[name], total), percent(sum, total),
			formatValue(cum[name], unit), percent(cum[name], total), name)
	}
	return b.String()
}

func formatValue(v int64, unit string) string {
	switch unit {
	case "nanoseconds":
		return fmt.Sprintf("%.2fs", float64(v)/float64(time.Second))
	case "bytes":
		return fmt.Sprintf("%.2fMB", float64(v)/(1<<20))
	default:
		return fmt.Sprintf("%d", v)
	}
}

func percent(v, total int64) float64 {
	if total == 0 {
		return 0
	}
	return float64(v) * 100 / float64(total)
}
