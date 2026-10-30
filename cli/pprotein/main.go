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

	pprofOpts := &collect.Options{
		Type:     "pprof",
		Ext:      "-pprof.pb.gz",
		Store:    store,
		EventHub: hub,
	}
	if err := pprof.NewHandler(pprofOpts).Register(api.Group("/pprof")); err != nil {
		return err
	}

	alpOpts := &collect.Options{
		Type:     "httplog",
		Ext:      "-httplog.log",
		Store:    store,
		EventHub: hub,
	}
	alpHandler, err := alp.NewHandler(alpOpts, store)
	if err != nil {
		return err
	}
	if err := alpHandler.Register(api.Group("/httplog")); err != nil {
		return err
	}

	slpOpts := &collect.Options{
		Type:     "slowlog",
		Ext:      "-slowlog.log",
		Store:    store,
		EventHub: hub,
	}
	slpHandler, err := slp.NewHandler(slpOpts, store)
	if err != nil {
		return err
	}
	if err := slpHandler.Register(api.Group("/slowlog")); err != nil {
		return err
	}

	memoOpts := &collect.Options{
		Type:     "memo",
		Ext:      "-memo.log",
		Store:    store,
		EventHub: hub,
	}
	if err := memo.NewHandler(memoOpts).Register(api.Group("/memo")); err != nil {
		return err
	}

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
