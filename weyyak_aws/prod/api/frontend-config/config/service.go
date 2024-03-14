package config

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"net/http"
)

type HandlerService struct{}

// All the services should be protected by auth token
func (hs *HandlerService) Bootstrap(r *gin.Engine) {
	// Setup Routes
	r.GET("/config", hs.GetAllConfig)

}

// GetAllConfig -  fetches all config
// GET /config
// @Summary Show a list of all country's
// @Description get list of all country's
// @Tags Config
// @Accept  json
// @Produce  json
// @Success 200 {array} object ConfigurationDetails
// @Router /config [get]
func (hs *HandlerService) GetAllConfig(c *gin.Context) {
	db := c.MustGet("DB").(*gorm.DB)
	var config ApplicationSetting
	if err := db.Debug().Where("name='ConfigEndpointResponseBody'").Find(&config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error(), "status": http.StatusInternalServerError})
	}

	var formated ConfigurationDetails
	if config.Value != "" {
		data := fmt.Sprintf("%v", config.Value)
		if err := json.Unmarshal([]byte(data), &formated); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error(), "Status": http.StatusInternalServerError})
		}
	}
	c.JSON(http.StatusOK, formated)
}