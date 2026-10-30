package memo

import (
	"fmt"
	"io"
	"net/http"

	"github.com/goccy/go-json"
	"github.com/labstack/echo/v4"
	"github.com/ymhiroki/pprotein-v2026/internal/collect"
)

type (
	textValue struct {
		Text string
	}

	requestBody struct {
		GroupId string
		Label   string
		Text    string
	}

	handler struct {
		opts      *collect.Options
		collector *collect.Collector
	}
)

func NewHandler(opts *collect.Options) *handler {
	return &handler{opts: opts}
}

func (h *handler) Register(g *echo.Group) error {
	p := &processor{}

	var err error
	h.collector, err = collect.New(p, h.opts)
	if err != nil {
		return fmt.Errorf("failed to initialize collector: %w", err)
	}

	g.GET("", h.getIndex)
	g.POST("", h.postIndex)
	g.GET("/:id", h.collector.HandleProcessed)
	return nil
}

func (h *handler) getIndex(c echo.Context) error {
	entries := h.collector.List()
	resp := make([]*collect.Entry, 0, len(entries))
	for _, ent := range entries {
		text, err := h.readText(ent.Snapshot.ID)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err)
		}

		// Respond with a copy so the entry shared with the collector is not modified.
		memo := *ent
		memo.Message = text
		resp = append(resp, &memo)
	}
	return c.JSON(http.StatusOK, resp)
}

// readText returns the memo text stored in the snapshot body.
func (h *handler) readText(id string) (string, error) {
	r, err := h.collector.Get(id)
	if err != nil {
		return "", fmt.Errorf("failed to get entry: %w", err)
	}
	defer r.Close()

	buf, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("failed to read entry: %w", err)
	}

	// A broken body shows up as an empty memo instead of failing the whole list.
	var v textValue
	_ = json.Unmarshal(buf, &v)
	return v.Text, nil
}

func (h *handler) postIndex(c echo.Context) error {
	req := &requestBody{}
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("failed to parse request body: %v", err))
	}

	target := &collect.SnapshotTarget{
		GroupId: req.GroupId,
		Label:   req.Label,
	}

	v := &textValue{
		Text: req.Text,
	}
	buf, err := json.Marshal(v)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Errorf("failed to marshal textValue: %v", err))
	}
	snapshot, err := h.collector.Add(target, buf)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("failed to add snapshot: %v", err))
	}

	eventData, err := json.Marshal(&collect.Entry{
		Snapshot: snapshot,
		Status:   "ok",
		Message:  req.Text,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Errorf("failed to marshal entry: %v", err))
	}
	h.opts.EventHub.Publish(eventData)

	return c.NoContent(http.StatusAccepted)
}
