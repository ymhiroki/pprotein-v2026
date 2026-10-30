package main

import (
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/ymhiroki/pprotein-v2026/integration/echov4"
	"github.com/ymhiroki/pprotein-v2026/internal/collect"
	"github.com/ymhiroki/pprotein-v2026/internal/collect/group"
	"github.com/ymhiroki/pprotein-v2026/internal/event"
	"github.com/ymhiroki/pprotein-v2026/internal/extproc/alp"
	"github.com/ymhiroki/pprotein-v2026/internal/extproc/slp"
	"github.com/ymhiroki/pprotein-v2026/internal/memo"
	"github.com/ymhiroki/pprotein-v2026/internal/pprof"
	"github.com/ymhiroki/pprotein-v2026/internal/report"
	"github.com/ymhiroki/pprotein-v2026/internal/storage"
	"github.com/ymhiroki/pprotein-v2026/view"
)

func start() error {
	port := os.Getenv("PORT")
	if port == "" {
		port = "9000"
	}

	store, err := storage.New("data")
	if err != nil {
		return err
	}

	e := echo.New()
	echov4.Integrate(e)

	fs, err := view.FS()
	if err != nil {
		return err
	}
	e.GET("/*", echo.WrapHandler(http.FileServer(http.FS(fs))))

	api := e.Group("/api", func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("Cache-Control", "no-store")
			return next(c)
		}
	})

	hub := event.NewHub()
	hub.RegisterHandlers(api.Group("/event"))

	// typ is the storage bucket and the Type shown in the UI; ext is the suffix of snapshot IDs.
	options := func(typ, ext string) *collect.Options {
		return &collect.Options{
			Type:     typ,
			Ext:      ext,
			Store:    store,
			EventHub: hub,
		}
	}

	pprofHandler := pprof.NewHandler(options("pprof", "-pprof.pb.gz"))
	if err := pprofHandler.Register(api.Group("/pprof")); err != nil {
		return err
	}

	alpHandler, err := alp.NewHandler(options("httplog", "-httplog.log"), store)
	if err != nil {
		return err
	}
	if err := alpHandler.Register(api.Group("/httplog")); err != nil {
		return err
	}

	slpHandler, err := slp.NewHandler(options("slowlog", "-slowlog.log"), store)
	if err != nil {
		return err
	}
	if err := slpHandler.Register(api.Group("/slowlog")); err != nil {
		return err
	}

	memoHandler := memo.NewHandler(options("memo", "-memo.log"))
	if err := memoHandler.Register(api.Group("/memo")); err != nil {
		return err
	}

	report.NewHandler(report.Sources{
		HTTPLog: alpHandler.Collector(),
		SlowLog: slpHandler.Collector(),
		PProf:   pprofHandler.Collector(),
		Memos:   memoHandler.Entries,
	}).Register(api.Group("/report"))

	grp, err := group.NewCollector(store, port)
	if err != nil {
		return err
	}
	grp.RegisterHandlers(api.Group("/group"))

	return e.Start(":" + port)
}

func main() {
	if err := start(); err != nil {
		panic(err)
	}
}
