package main

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof" //nolint
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/vmihub/config"
	"github.com/projecteru2/vmihub/internal/api"
	"github.com/projecteru2/vmihub/internal/models"
	storFact "github.com/projecteru2/vmihub/internal/storage/factory"
	"github.com/projecteru2/vmihub/internal/utils"
	myvalidator "github.com/projecteru2/vmihub/internal/validator"
	"github.com/projecteru2/vmihub/internal/version"
	zerolog "github.com/rs/zerolog/log"
	cli "github.com/urfave/cli/v2"
)

var (
	configPath string
)

func main() {
	cli.VersionPrinter = func(_ *cli.Context) {
		fmt.Print(version.String())
	}

	app := cli.NewApp()
	app.Name = version.NAME
	app.Usage = "Run vmihub"
	app.Version = version.VERSION
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:        "config",
			Value:       "/etc/eru/vmihub.toml",
			Usage:       "config file path for vmihub, in toml",
			Destination: &configPath,
			EnvVars:     []string{"ERU_vmihub_CONFIG_PATH"},
		},
	}
	app.Commands = []*cli.Command{
		{
			Name:   "server",
			Usage:  "run vmihub server",
			Action: runServer,
		},
	}
	app.Action = runServer
	_ = app.Run(os.Args)
}

func prepare(_ context.Context, cfg *config.Config) error {
	if err := models.Init(&cfg.Mysql, nil); err != nil {
		return err
	}
	if _, err := storFact.Init(&cfg.Storage); err != nil {
		return err
	}
	utils.SetupRedis(&cfg.Redis, nil)

	return nil
}

// @title vmihub project
// @version 1.0
// @description this is vmihub server.
// @BasePath /api/v1
func runServer(_ *cli.Context) error {
	cfg, err := config.Init(configPath)
	if err != nil {
		zerolog.Fatal().Err(err).Send()
	}
	// kill -9 is syscall. SIGKILL but can"t be catch, so don't need add it
	ctx, cancel := signal.NotifyContext(context.TODO(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	logCfg := &types.ServerLogConfig{
		Level:      cfg.Log.Level,
		UseJSON:    cfg.Log.UseJSON,
		Filename:   cfg.Log.Filename,
		MaxSize:    cfg.Log.MaxSize,
		MaxAge:     cfg.Log.MaxAge,
		MaxBackups: cfg.Log.MaxBackups,
	}
	if err := log.SetupLog(ctx, logCfg, cfg.Log.SentryDSN); err != nil {
		zerolog.Fatal().Err(err).Send()
	}
	defer log.SentryDefer()

	if err := prepare(ctx, cfg); err != nil {
		log.WithFunc("main").Error(ctx, err, "Can't init server")
		return err
	}

	gin.SetMode(cfg.Server.RunMode)
	routersInit, err := api.SetupRouter()
	if err != nil {
		return err
	}
	readTimeout := cfg.Server.ReadTimeout
	writeTimeout := cfg.Server.WriteTimeout
	endPoint := cfg.Server.Bind

	maxHeaderBytes := 1 << 20

	log.WithFunc("main").Infof(ctx, "config info %s", cfg.String())

	srv := &http.Server{
		Addr:           endPoint,
		Handler:        routersInit,
		ReadTimeout:    readTimeout,
		WriteTimeout:   writeTimeout,
		MaxHeaderBytes: maxHeaderBytes,
	}
	log.Infof(ctx, "start http server listening %s", cfg.Server.Bind)

	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		_ = v.RegisterValidation("email", myvalidator.ValidateEmail)
	}

	go handleSignals(ctx, func(ctx context.Context) {
		if err := srv.Shutdown(ctx); err != nil {
			log.Errorf(ctx, err, "Server Shutdown:")
		}
	})

	err = srv.ListenAndServe()
	if err != nil {
		log.Error(ctx, err, "Error when running server")
	}
	return nil
}

func handleSignals(ctx context.Context, shutdownFn func(context.Context)) {
	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 5 seconds.
	<-ctx.Done()

	newCtx, cancel := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancel()

	log.Info(context.TODO(), "Shutdown Server ...")
	shutdownFn(newCtx)
	log.Info(context.TODO(), "Server exited")
}
