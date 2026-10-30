package collect

import (
	"fmt"
	"net/http"
	"slices"

	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
)

// RegisterHandlers registers the endpoints shared by all collector types:
//
//	GET  ""             list entries
//	POST ""             start collecting a snapshot in the background
//	GET  /data/:id      download the raw body of a snapshot
//	GET  /data/latest   download the raw body of the newest successful snapshot (?label= to filter)
func (c *Collector) RegisterHandlers(g *echo.Group) {
	g.GET("", c.handleList)
	g.POST("", c.handleCollect)
	g.GET("/data/:id", c.handleData)
	g.GET("/data/latest", c.handleLatestData)
}

// HandleProcessed streams the processor output of the snapshot given by the :id path parameter.
func (c *Collector) HandleProcessed(ctx echo.Context) error {
	r, err := c.Get(ctx.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Errorf("failed to get entry: %w", err))
	}
	defer r.Close()

	return ctx.Stream(http.StatusOK, "application/json", r)
}

func (c *Collector) handleList(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, c.List())
}

func (c *Collector) handleCollect(ctx echo.Context) error {
	target := &SnapshotTarget{}
	if err := ctx.Bind(target); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("failed to parse request body: %v", err))
	}

	go func() {
		if err := c.Collect(target); err != nil {
			log.Error("[!] collector aborted:", err)
		}
	}()

	return ctx.NoContent(http.StatusOK)
}

func (c *Collector) handleData(ctx echo.Context) error {
	ent, ok := c.lookup(ctx.Param("id"))
	if !ok {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	return serveBody(ctx, ent.Snapshot)
}

func (c *Collector) handleLatestData(ctx echo.Context) error {
	label := ctx.QueryParam("label")

	entries := c.List()
	slices.SortFunc(entries, func(a, b *Entry) int {
		return b.Snapshot.Datetime.Compare(a.Snapshot.Datetime)
	})

	for _, ent := range entries {
		if label != "" && label != ent.Snapshot.Label {
			continue
		}
		if ent.Status != StatusOk {
			continue
		}
		return serveBody(ctx, ent.Snapshot)
	}
	return echo.NewHTTPError(http.StatusNotFound)
}

func serveBody(ctx echo.Context, snapshot *Snapshot) error {
	bodyPath, err := snapshot.BodyPath()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to get body path: %v", err))
	}
	return ctx.File(bodyPath)
}
