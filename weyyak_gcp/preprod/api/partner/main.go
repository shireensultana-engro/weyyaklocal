package main

import (
	"fmt"
	// "goroach/handler"
	"masterdata/content"

	"masterdata/docs"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"github.com/rs/zerolog"
	_ "github.com/rs/zerolog/log"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

var (
	db     *gorm.DB
	router *gin.Engine
	log    zerolog.Logger
)

// @securityDefinitions.apikey Authorization
// @in header
// @name Authorization
func main() {
	// Initialize Dependencies
	// Service Port, Database, Logger, Cache, Message Queue etc.
	router := gin.Default()
	router.Use(CORSMiddleware())
	log := zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout, NoColor: false})
	// Database
	// dsn := fmt.Sprintf("host=%s port=%s user=%s dbname=%s sslmode=disable password=%s",
	// 	os.Getenv("DB_SERVER"), os.Getenv("DB_PORT"), os.Getenv("DB_USER"),
	// 	os.Getenv("DB_DATABASE"), os.Getenv("DB_PASSWORD"),
	// )
	dsn := "postgres://" + os.Getenv("DB_USER") + ":" + os.Getenv("DB_PASSWORD") + "@" + os.Getenv("DB_SERVER") + ":" + os.Getenv("DB_PORT") + "/" + os.Getenv("DB_DATABASE")
	// log.Info().Msg(dsn)
	db, err := gorm.Open("postgres", dsn)
	if err != nil {
		log.Error().Err(err).Msg("")
	}
	defer db.Close()
	fdsn := "postgres://" + os.Getenv("DB_USER") + ":" + os.Getenv("DB_PASSWORD") + "@" + os.Getenv("DB_SERVER") + ":" + os.Getenv("DB_PORT") + "/" + os.Getenv("FRONTEND_DB_DATABASE")
	// log.Info().Msg(dsn)
	fdb, err := gorm.Open("postgres", fdsn)
	if err != nil {
		log.Error().Err(err).Msg("")
	}
	defer fdb.Close()
	fcdsn := "postgres://" + os.Getenv("DB_USER") + ":" + os.Getenv("DB_PASSWORD") + "@" + os.Getenv("DB_SERVER") + ":" + os.Getenv("DB_PORT") + "/wyk_frontend_config"
	// log.Info().Msg(dsn)
	fcdb, err := gorm.Open("postgres", fcdsn)
	if err != nil {
		log.Error().Err(err).Msg("")
	}
	defer fdb.Close()
	ucdsn := "postgres://" + os.Getenv("DB_USER") + ":" + os.Getenv("DB_PASSWORD") + "@" + os.Getenv("DB_SERVER") + ":" + os.Getenv("DB_PORT") + "/wk_user_management"
	// log.Info().Msg(dsn)
	udb, err := gorm.Open("postgres", ucdsn)
	if err != nil {
		log.Error().Err(err).Msg("")
	}
	defer udb.Close()
	// db.LogMode(true)
	db.SingularTable(true)
	db.DB().SetMaxIdleConns(10)
	fdb.DB().SetMaxIdleConns(10)
	fcdb.DB().SetMaxIdleConns(10)
	udb.DB().SetMaxIdleConns(10)
	fdb.SingularTable(true)
	fcdb.SingularTable(true)
	udb.SingularTable(true)
	// Swagger info
	docs.SwaggerInfo.Title = "Weyyak Third Party APIs"
	docs.SwaggerInfo.Description = "Third Party APIs"
	docs.SwaggerInfo.Version = "1.0"
	docs.SwaggerInfo.Host = "msapiprod-me-partner.z5.com"
	docs.SwaggerInfo.BasePath = "/"
	docs.SwaggerInfo.Schemes = []string{"https", "http"}

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Setup Middleware for Database and Log
	router.Use(func(c *gin.Context) {
		c.Set("DB", db)
		c.Set("FDB", fdb)
		c.Set("FCDB", fcdb)
		c.Set("UDB", udb)
		c.Set("LOG", log)
		c.Set("REDIS", "redis")
	})

	// Boostrap services
	episodeSvc := &content.HandlerService{}
	episodeSvc.Bootstrap(router)
	// --- Development Only ---
	//setupQuotes(db)

	// Start the service
	router.GET("/health", healthsvc)
	port := os.Getenv("SERVICE_PORT")
	log.Info().Msg("Starting server on :" + port)
	router.Run(":" + port)
}

func healthsvc(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": health()})
}

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "http://localhost:3006")
		c.Writer.Header().Set("Access-Control-Max-Age", "86400")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, UPDATE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Access-Control-Allow-Origin, Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		c.Writer.Header().Set("Access-Control-Expose-Headers", "Content-Length")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")

		//fmt.Println(c.Request.Method)

		if c.Request.Method == "OPTIONS" {
			fmt.Println("OPTIONS")
			c.AbortWithStatus(200)
		} else {
			c.Next()
		}
	}
}