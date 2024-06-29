package middlewares

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/projecteru2/core/log"
)

type ginHands struct {
	SerName    string
	Path       string
	Latency    time.Duration
	Method     string
	StatusCode int
	ClientIP   string
	MsgStr     string
}

func ErrorLogger() gin.HandlerFunc {
	return ErrorLoggerT(gin.ErrorTypeAny)
}

func ErrorLoggerT(typ gin.ErrorType) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if !c.Writer.Written() {
			json := c.Errors.ByType(typ).JSON()
			if json != nil {
				c.JSON(-1, json)
			}
		}
	}
}

func Logger(serName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		t := time.Now()
		// before request
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery
		c.Next()
		// after request
		// latency := time.Since(t)
		// clientIP := c.ClientIP()
		// method := c.Request.Method
		// statusCode := c.Writer.Status()
		if raw != "" {
			path = path + "?" + raw
		}
		msg := c.Errors.String()
		if msg == "" {
			msg = "Request"
		}
		cData := &ginHands{
			SerName:    serName,
			Path:       path,
			Latency:    time.Since(t),
			Method:     c.Request.Method,
			StatusCode: c.Writer.Status(),
			ClientIP:   c.ClientIP(),
			MsgStr:     msg,
		}

		logSwitch(cData)
	}
}

func logSwitch(data *ginHands) {
	switch {
	case data.StatusCode >= 400 && data.StatusCode < 500:
		log.GetGlobalLogger().Warn().
			Str("ser_name", data.SerName).
			Str("method", data.Method).
			Str("path", data.Path).
			Dur("resp_time", data.Latency).
			Int("status", data.StatusCode).
			Str("client_ip", data.ClientIP).
			Msg(data.MsgStr)

	case data.StatusCode >= http.StatusInternalServerError:
		log.GetGlobalLogger().Error().
			Str("ser_name", data.SerName).
			Str("method", data.Method).
			Str("path", data.Path).
			Dur("resp_time", data.Latency).
			Int("status", data.StatusCode).
			Str("client_ip", data.ClientIP).
			Msg(data.MsgStr)

	default:
		log.GetGlobalLogger().Info().
			Str("ser_name", data.SerName).
			Str("method", data.Method).
			Str("path", data.Path).
			Dur("resp_time", data.Latency).
			Int("status", data.StatusCode).
			Str("client_ip", data.ClientIP).
			Msg(data.MsgStr)
	}
}
