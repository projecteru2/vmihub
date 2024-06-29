package middlewares

import ( //nolint:goimports
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"time" //nolint:goimports
)

func Cors() gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "X-Requested-With", "Content-Type", "Accept", "X-CSRF-TOKEN", "Authorization"},
		ExposeHeaders:    []string{"Content-Length", "Content-Disposition"},
		AllowCredentials: true,
		// AllowOriginFunc: func(origin string) bool {
		// 	allowOrigins := map[string]bool {
		// 		"http://localhost:3000": true,
		// 	}
		// 	_, ok := allowOrigins[origin]
		// 	return ok
		// },
		MaxAge: 1 * time.Hour,
	})
}
