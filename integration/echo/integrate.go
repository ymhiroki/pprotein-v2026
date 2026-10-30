package echo

import (
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/labstack/gommon/log"
	"github.com/ymhiroki/pprotein-v2026/integration"
)

func Integrate(e *echo.Echo) {
	EnableDebugHandler(e)
	EnableDebugMode(e)
}

func EnableDebugHandler(e *echo.Echo) {
	e.Any("/debug/*", echo.WrapHandler(integration.NewDebugHandler()))
}

func EnableDebugMode(e *echo.Echo) {
	e.Debug = true
	e.Logger.SetLevel(log.DEBUG)
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
}
