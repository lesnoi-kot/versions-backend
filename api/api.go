package api

import (
	"context"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/lesnoi-kot/versions-backend/mongostore"
	"github.com/lesnoi-kot/versions-backend/mq"
)

const (
	SHUTDOWN_TIMEOUT = 10_000
)

type APIConfig struct {
	Store        *mongostore.Store
	MQ           *mq.AMQPConnection
	AllowOrigins []string
	Debug        bool
}

type APIService struct {
	APIConfig
	handler *echo.Echo
}

func NewAPI(config APIConfig) *APIService {
	api := &APIService{
		APIConfig: config,
		handler:   echo.New(),
	}

	api.handler.Debug = config.Debug
	api.handler.HTTPErrorHandler = api.errorHandler

	securityConfig := middleware.SecureConfig{
		ContentSecurityPolicy: "default-src 'none';",
		ReferrerPolicy:        "same-origin",
	}

	corsConfig := middleware.CORSConfig{
		AllowOrigins:     config.AllowOrigins,
		AllowCredentials: false, // Allow cookies in cross origin requests.
	}

	api.handler.Pre(middleware.RemoveTrailingSlash())
	api.handler.Use(
		middleware.Logger(),
		middleware.SecureWithConfig(securityConfig),
		middleware.CORSWithConfig(corsConfig),
		middleware.BodyLimit("1M"),
	)

	if !config.Debug {
		api.handler.Use(middleware.Recover())
	}

	initRoutes(api)
	return api
}

func initRoutes(api *APIService) {
	root := api.handler.Group("")

	sources := root.Group("/sources")
	sources.GET("", api.getSources)
	sources.GET("/:id", api.getSource)
	sources.POST("", api.addSource)
}

func (api *APIService) errorHandler(err error, c echo.Context) {
	api.handler.DefaultHTTPErrorHandler(err, c)
}

func (a APIService) Start(address string) error {
	return a.handler.Start(address)
}

func (a APIService) Shutdown() error {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		SHUTDOWN_TIMEOUT*time.Millisecond,
	)

	defer cancel()
	return a.handler.Shutdown(ctx)
}
