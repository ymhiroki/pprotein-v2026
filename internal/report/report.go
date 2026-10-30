// Package report renders the results of one collection group as a single Markdown
// document, to read in a terminal or to hand to an AI assistant.
package report

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/ymhiroki/pprotein-v2026/internal/collect"
	"github.com/ymhiroki/pprotein-v2026/internal/pprof"
)

const (
	defaultRows = 20
	maxRows     = 1000
)

type (
	// Sources are the collectors whose entries make up a report.
	Sources struct {
		HTTPLog *collect.Collector
		SlowLog *collect.Collector
		PProf   *collect.Collector
		// Memos returns the memo entries with Message set to the memo text.
		Memos func() ([]*collect.Entry, error)
	}

	Handler struct {
		src Sources
	}
)

func NewHandler(src Sources) *Handler {
	return &Handler{src: src}
}

// Register registers GET /latest for the newest group and GET /:gid for any group.
// Both take ?n= for the number of rows in each table.
func (h *Handler) Register(g *echo.Group) {
	g.GET("/latest", h.handleLatest)
	g.GET("/:gid", h.handleGroup)
}

func (h *Handler) handleLatest(c echo.Context) error {
	groups := h.groups()
	if len(groups) == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "no group has been collected yet")
	}
	return h.serve(c, groups[len(groups)-1], groups)
}

func (h *Handler) handleGroup(c echo.Context) error {
	gid := c.Param("gid")
	groups := h.groups()
	if !slices.Contains(groups, gid) {
		return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("no such group: %s", gid))
	}
	return h.serve(c, gid, groups)
}

func (h *Handler) serve(c echo.Context, gid string, groups []string) error {
	n := defaultRows
	if s := c.QueryParam("n"); s != "" {
		v, err := strconv.Atoi(s)
		if err != nil || v < 1 || v > maxRows {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("n must be a number from 1 to %d", maxRows))
		}
		n = v
	}

	doc, err := h.render(gid, groups, n)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.Blob(http.StatusOK, "text/plain; charset=utf-8", []byte(doc))
}

// groups returns the IDs of all collection groups, oldest first. Group IDs are
// timestamps, so sorting them as strings sorts them by time.
func (h *Handler) groups() []string {
	seen := map[string]bool{}
	for _, c := range []*collect.Collector{h.src.HTTPLog, h.src.SlowLog, h.src.PProf} {
		for _, ent := range c.List() {
			if gid := ent.Snapshot.GroupId; gid != "" {
				seen[gid] = true
			}
		}
	}
	groups := make([]string, 0, len(seen))
	for gid := range seen {
		groups = append(groups, gid)
	}
	sort.Strings(groups)
	return groups
}

func (h *Handler) render(gid string, groups []string, n int) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "# pprotein report: group %s\n\n", gid)
	if i := slices.Index(groups, gid); i > 0 {
		fmt.Fprintf(&b, "Previous group: %s (GET /api/report/%s)\n\n", groups[i-1], groups[i-1])
	}

	httplogs := entriesOf(h.src.HTTPLog, gid)
	slowlogs := entriesOf(h.src.SlowLog, gid)
	pprofs := entriesOf(h.src.PProf, gid)

	b.WriteString("## Entries\n\n")
	b.WriteString("| Type | Label | Status | Commit | URL |\n| --- | --- | --- | --- | --- |\n")
	for _, ent := range slices.Concat(httplogs, slowlogs, pprofs) {
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", ent.Snapshot.Type, cell(ent.Snapshot.Label),
			cell(status(ent)), cell(commit(ent)), cell(ent.Snapshot.URL))
	}

	memos, err := h.src.Memos()
	if err != nil {
		return "", fmt.Errorf("failed to read memos: %w", err)
	}
	memos = slices.DeleteFunc(memos, func(m *collect.Entry) bool { return m.Snapshot.GroupId != gid })
	sort.SliceStable(memos, func(i, j int) bool { return memos[i].Snapshot.Datetime.Before(memos[j].Snapshot.Datetime) })
	if len(memos) > 0 {
		b.WriteString("\n## Memos\n\n")
		for _, m := range memos {
			label := m.Snapshot.Label
			if label == "" {
				label = "(no label)"
			}
			fmt.Fprintf(&b, "- %s\n", label)
			for _, line := range strings.Split(strings.TrimRight(m.Message, "\n"), "\n") {
				fmt.Fprintf(&b, "  %s\n", line)
			}
		}
	}

	for _, ent := range httplogs {
		writeTSV(&b, h.src.HTTPLog, ent, "alp", "Sum", "Times are in seconds and body sizes in bytes.", n)
	}
	for _, ent := range slowlogs {
		writeTSV(&b, h.src.SlowLog, ent, "slp", "Sum(QueryTime)", "Times are in seconds.", n)
	}
	for _, ent := range pprofs {
		writeProfile(&b, ent, n)
	}
	return b.String(), nil
}

// entriesOf returns the entries of the group, sorted by label.
func entriesOf(c *collect.Collector, gid string) []*collect.Entry {
	entries := slices.DeleteFunc(c.List(), func(ent *collect.Entry) bool { return ent.Snapshot.GroupId != gid })
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Snapshot.Label != entries[j].Snapshot.Label {
			return entries[i].Snapshot.Label < entries[j].Snapshot.Label
		}
		return entries[i].Snapshot.Datetime.Before(entries[j].Snapshot.Datetime)
	})
	return entries
}

// writeTSV writes the top n rows of the alp or slp result, sorted by the sortBy column.
func writeTSV(b *strings.Builder, c *collect.Collector, ent *collect.Entry, tool, sortBy, units string, n int) {
	fmt.Fprintf(b, "\n## %s: %s\n\n", ent.Snapshot.Type, ent.Snapshot.Label)
	if ent.Status != collect.StatusOk {
		fmt.Fprintf(b, "No data: %s\n", status(ent))
		return
	}

	r, err := c.Get(ent.Snapshot.ID)
	if err != nil {
		fmt.Fprintf(b, "Failed to get the %s result: %v\n", tool, err)
		return
	}
	raw, err := io.ReadAll(r)
	r.Close()
	if err != nil {
		fmt.Fprintf(b, "Failed to read the %s result: %v\n", tool, err)
		return
	}

	header, rows := splitTSV(string(raw))
	shown := min(n, len(rows))
	if col := slices.Index(header, sortBy); col >= 0 {
		sort.SliceStable(rows, func(i, j int) bool { return number(rows[i], col) > number(rows[j], col) })
		fmt.Fprintf(b, "%s result, top %d of %d rows by %s. %s\n\n", tool, shown, len(rows), sortBy, units)
	} else {
		fmt.Fprintf(b, "%s result, first %d of %d rows. %s\n\n", tool, shown, len(rows), units)
	}

	b.WriteString("```tsv\n")
	b.WriteString(strings.Join(header, "\t") + "\n")
	for _, row := range rows[:shown] {
		b.WriteString(strings.Join(row, "\t") + "\n")
	}
	b.WriteString("```\n")
}

func splitTSV(s string) (header []string, rows [][]string) {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if header == nil {
			header = fields
		} else {
			rows = append(rows, fields)
		}
	}
	return header, rows
}

// number returns the value of column col, or -Inf if it is missing or not a number.
func number(row []string, col int) float64 {
	if col >= len(row) {
		return math.Inf(-1)
	}
	v, err := strconv.ParseFloat(row[col], 64)
	if err != nil {
		return math.Inf(-1)
	}
	return v
}

func writeProfile(b *strings.Builder, ent *collect.Entry, n int) {
	fmt.Fprintf(b, "\n## pprof: %s\n\n", ent.Snapshot.Label)
	if ent.Status != collect.StatusOk {
		fmt.Fprintf(b, "No data: %s\n", status(ent))
		return
	}

	path, err := ent.Snapshot.BodyPath()
	if err != nil {
		fmt.Fprintf(b, "Failed to find the profile: %v\n", err)
		return
	}
	s, err := pprof.Summarize(path, n)
	if err != nil {
		fmt.Fprintf(b, "Failed to read the profile: %v\n", err)
		return
	}

	fmt.Fprintf(b, "Type: %s", s.SampleType)
	if s.Duration > 0 {
		fmt.Fprintf(b, ", Duration: %s", s.Duration.Round(100*time.Millisecond))
	}
	fmt.Fprintf(b, ", Total: %s\n\n", s.Total)
	fmt.Fprintf(b, "Top %d of %d functions by flat:\n\n```\n%s```\n\n", s.Shown, s.Functions, s.ByFlat)
	fmt.Fprintf(b, "Top %d of %d functions by cum:\n\n```\n%s```\n", s.Shown, s.Functions, s.ByCum)
}

func status(ent *collect.Entry) string {
	if ent.Status == collect.StatusOk {
		return "ok"
	}
	return fmt.Sprintf("%s: %s", ent.Status, ent.Message)
}

func commit(ent *collect.Entry) string {
	repo := ent.Snapshot.Repository
	if repo == nil || repo.Hash == "" {
		return "-"
	}
	msg, _, _ := strings.Cut(repo.Message, "\n")
	return fmt.Sprintf("%.7s %s", repo.Hash, msg)
}

// cell makes s safe to put in a Markdown table cell.
func cell(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.ReplaceAll(s, "|", `\|`)
}
