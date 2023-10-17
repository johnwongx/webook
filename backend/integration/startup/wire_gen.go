// Code generated by Wire. DO NOT EDIT.

//go:generate go run github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package startup

import (
	"github.com/gin-gonic/gin"
	"github.com/google/wire"
	"github.com/johnwongx/webook/backend/internal/repository"
	"github.com/johnwongx/webook/backend/internal/repository/cache"
	"github.com/johnwongx/webook/backend/internal/repository/dao"
	"github.com/johnwongx/webook/backend/internal/service"
	"github.com/johnwongx/webook/backend/internal/web"
	"github.com/johnwongx/webook/backend/internal/web/jwt"
	"github.com/johnwongx/webook/backend/ioc"
)

// Injectors from wire.go:

func InitWebServer() *gin.Engine {
	cmdable := InitRedis()
	limiter := ioc.InitRedisRateLimit(cmdable)
	jwtHandler := jwt.NewRedisJwtHandler(cmdable)
	logger := InitLog()
	v := ioc.InitMiddlewares(limiter, jwtHandler, logger)
	gormDB := InitTestDB()
	userDAO := dao.NewUserDAO(gormDB)
	userCache := cache.NewRedisUserCache(cmdable)
	userRepository := repository.NewUserRepository(userDAO, userCache)
	userService := service.NewUserService(userRepository, logger)
	smsService := ioc.InitLocalSms()
	codeCache := cache.NewRedisCodeCache(cmdable)
	codeRepository := repository.NewCodeRepository(codeCache)
	codeService := service.NewCodeService(smsService, codeRepository)
	userHandler := web.NewUserHandler(userService, codeService, logger, jwtHandler)
	wechatService := InitPhantomWechatService(logger)
	wechatHandlerConfig := ioc.NewWechatHandlerConfig()
	oAuth2WechatHandler := web.NewWechatHandler(wechatService, userService, wechatHandlerConfig)
	articleDAO := dao.NewGORMArticleDAO(gormDB)
	articleRepository := repository.NewArticleRepository(articleDAO)
	articleService := service.NewArticleService(articleRepository, logger)
	articleHandler := web.NewArticleHandler(articleService, logger)
	engine := ioc.InitWebServer(v, userHandler, oAuth2WechatHandler, articleHandler)
	return engine
}

func InitArticleHandler() *web.ArticleHandler {
	gormDB := InitTestDB()
	articleDAO := dao.NewGORMArticleDAO(gormDB)
	articleRepository := repository.NewArticleRepository(articleDAO)
	logger := InitLog()
	articleService := service.NewArticleService(articleRepository, logger)
	articleHandler := web.NewArticleHandler(articleService, logger)
	return articleHandler
}

// wire.go:

var thirdProvider = wire.NewSet(InitRedis, InitTestDB, InitLog)

var userSvcProvider = wire.NewSet(dao.NewUserDAO, cache.NewRedisUserCache, repository.NewUserRepository, service.NewUserService)

var articleSvcProvider = wire.NewSet(dao.NewGORMArticleDAO, repository.NewArticleRepository, service.NewArticleService)
