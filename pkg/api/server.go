package api

import (
	"github.com/huo-ju/dfserver/pkg/rabbitmq"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	echolog "github.com/labstack/gommon/log"
)

func StartApiServer(jwtSecret string, port string, q *rabbitmq.AmqpQueue) error {
	e := echo.New()
	e.HideBanner = true
	e.Binder = new(CustomBinder)
	e.Use(middleware.Logger())
	e.Logger.SetLevel(echolog.DEBUG)

	r := e.Group("/api")
	r.Use(middleware.JWTWithConfig(middleware.JWTConfig{
		SigningKey:  []byte(jwtSecret),
		TokenLookup: "cookie:Authorization",
	}))

	r.POST("/task", CreateTask(q))
	err := e.Start(port)
	return err
}
