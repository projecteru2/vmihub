package api

import (
	"context"
	"net/http"

	ginI18n "github.com/gin-contrib/i18n"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/projecteru2/vmihub/assets"
	"github.com/projecteru2/vmihub/internal/api/image"
	"github.com/projecteru2/vmihub/internal/api/user"
	"github.com/projecteru2/vmihub/internal/middlewares"
	"github.com/projecteru2/vmihub/internal/utils"
	"github.com/projecteru2/vmihub/internal/utils/redissession"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

// SetupRouter initialize routing information
func SetupRouter() (*gin.Engine, error) {
	r := gin.New()
	redisCli := utils.GetRedisConn()
	sessStor, err := redissession.NewStore(context.TODO(), redisCli)
	if err != nil {
		return nil, err
	}
	//// 设置 session 的最大存活时间
	//  sessStor.Options(sessions.Options{
	//	MaxAge:   7200, // 有效期为2小时
	//	HttpOnly: true,
	//	Secure:   false, // 如果是在 HTTPS 环境下应设为 true
	//	})
	r.Use(sessions.Sessions("mysession", sessStor))

	r.Use(ginI18n.Localize(ginI18n.WithBundle(&ginI18n.BundleCfg{
		RootPath:         "./i18n/localize",
		AcceptLanguage:   []language.Tag{language.Chinese, language.English},
		DefaultLanguage:  language.Chinese,
		UnmarshalFunc:    yaml.Unmarshal,
		FormatBundleFile: "yaml",
		Loader:           &ginI18n.EmbedLoader{FS: assets.Assets},
	})))

	r.Use(middlewares.Cors())
	r.Use(middlewares.Logger("vmihub"))

	r.GET("/healthz", func(c *gin.Context) {
		c.String(http.StatusOK, ginI18n.MustGetMessage(c, "healthy"))
	})

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	basePath := "/api/v1"
	apiGroup := r.Group(basePath, middlewares.Authenticate())

	image.SetupRouter(apiGroup)
	user.SetupRouter(basePath, r)
	return r, nil
}
