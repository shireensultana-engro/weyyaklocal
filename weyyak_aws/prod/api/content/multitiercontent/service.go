package multitiercontent

import (
	"bytes"
	"content/common"
	"content/fragments"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	gormbulk "github.com/t-tiger/gorm-bulk-insert/v2"
	"github.com/thanhpk/randstr"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

type HandlerService struct{}

func (hs *HandlerService) Bootstrap(r *gin.Engine) {
	qrg := r.Group("/api/contents")
	qrg.Use(common.ValidateToken())
	qrg.GET("/multitier/titles", hs.MultitierContent)
	qrg.DELETE("/onetier/:result", hs.DeleteOnetierorMultitier)
	qrg.DELETE("/multitier/:result", hs.DeleteOnetierorMultitier)
	qrg.POST("/onetier/:id/status/:status", hs.UpdateOnetier)
	qrg.POST("/multitier/:id/status/:status", hs.UpdateMultitier)

	qrg.POST("/multitier/published", hs.CreateOrUpdatePublishedMultitierContentDetails)
	qrg.POST("/multitier/published/:id", hs.CreateOrUpdatePublishedMultitierContentDetails)
	qrg.POST("/multitier/draft", hs.CreateOrUpdateDraftMultitierContentDetails)
	qrg.POST("/multitier/draft/:id", hs.CreateOrUpdateDraftMultitierContentDetails)
	qrg.POST("/onetier/published", hs.CreateOrUpdatePublishedOnetierContentDetails)
	qrg.POST("/onetier/published/:id", hs.CreateOrUpdatePublishedOnetierContentDetails)
	qrg.POST("/onetier/draft", hs.CreateOrUpdateDraftOnetierContentDetails)
	qrg.POST("/onetier/draft/:id", hs.CreateOrUpdateDraftOnetierContentDetails)

}

// Get Multitier Content With Tittles -fetches Multitier Content With Tittles
// GET /api/contents/multitier/titles
// @Summary Show Multitier Content With Tittles
// @Description get Multitier Content With Tittles
// @Tags onetier or multitier
// @security Authorization
// @Accept  json
// @Produce  json
// @Success 200 {array} Multitier
// @Router /api/contents/multitier/titles [get]
func (hs *HandlerService) MultitierContent(c *gin.Context) {
	db := c.MustGet("DB").(*gorm.DB)
	var result []Multitier
	if c.MustGet("AuthorizationRequired") == 1 || c.MustGet("is_back_office_user") == false {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Authorization has been denied for this request."})
		return
	}
	var erroresponse = common.ServerErrorResponse()
	if err := db.Debug().Table("content_primary_info cpi").
		Select("cpi.transliterated_title,c.id").Joins("join content c on c.primary_info_id=cpi.id").
		Where("c.content_type='Series' or c.content_type='Program' and c.deleted_by_user_id is null").Find(&result).Error; err != nil {
		c.JSON(http.StatusInternalServerError, erroresponse)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

// For  Update onetier status-Update onetier status
// POST /api/contents/onetier/{id}/status/{status}
// @Summary Show Update onetier status
// @Description post Update onetier status
// @Tags onetier
// @Accept  json
// @Produce  json
// @security Authorization
// @Param tier path string true "tier"
// @Param id path string true "Id"
// @Param status path string true "status"
// @Success 200 {array} object c.JSON
// @Router /api/contents/onetier/{id}/status/{status} [post]
func (hs *HandlerService) UpdateOnetier(c *gin.Context) {
	db := c.MustGet("DB").(*gorm.DB)
	var errorFlag bool
	errorFlag = false
	var invalidError common.InvalidError
	var invalid common.Invalids
	var finalErrorResponse common.FinalErrorResponse
	var contenttype ContentType
	if c.MustGet("AuthorizationRequired") == 1 || c.MustGet("is_back_office_user") == false {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Authorization has been denied for this request."})
		return
	}
	var count int
	db.Debug().Table("content").Where("id=? and deleted_by_user_id is null", c.Param("id")).Count(&count)
	if count > 0 {
		db.Debug().Table("content").Where("id=? and deleted_by_user_id is null", c.Param("id")).Update(UpdateStatus{Status: c.Param("status"), ModifiedAt: time.Now()})
		// db.Table("content_variance cv").Where("cv.content_id = ? and cv.deleted_by_user_id is null", c.Param("id")).Update(UpdateStatus{Status: c.Param("status"), ModifiedAt: time.Now()})
		res := map[string]string{
			"id": c.Param("id"),
		}
		db.Debug().Raw("select content_type from content where id=?", c.Param("id")).Find(&contenttype)
		go common.CreateRedisKeyForContentType(contenttype.ContentType, c)
		c.JSON(http.StatusOK, gin.H{"data": res})
		return
	} else {
		errorFlag = true
		invalidError = common.InvalidError{Code: "error_content_not_found", Description: "The specified condition was not met for 'Id'."}

		invalid.Id = invalidError

		finalErrorResponse = common.FinalErrorResponse{Error: "not_found", Description: "Not found.", Code: "", RequestId: randstr.String(32), Invalid: invalid}

	}
	if errorFlag {
		c.JSON(http.StatusBadRequest, finalErrorResponse)
		return
	}
}

// For  Update multitier status-Update multitier status
// POST /api/contents/onetier/{id}/status/{status}
// @Summary Show Update multitier status
// @Description post Update onetier or multitier status
// @Tags multitier
// @Accept  json
// @Produce  json
// @security Authorization
// @Param tier path string true "tier"
// @Param id path string true "Id"
// @Param status path string true "status"
// @Success 200 {array} object c.JSON
// @Router /api/contents/multitier/{id}/status/{status} [post]
func (hs *HandlerService) UpdateMultitier(c *gin.Context) {
	db := c.MustGet("DB").(*gorm.DB)
	var errorFlag bool
	errorFlag = false
	var invalidError common.InvalidError
	var invalid common.Invalids
	var finalErrorResponse common.FinalErrorResponse
	var contenttype ContentType
	if c.MustGet("AuthorizationRequired") == 1 || c.MustGet("is_back_office_user") == false {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Authorization has been denied for this request."})
		return
	}
	var count int
	db.Debug().Table("content").Where("id=? and deleted_by_user_id is null", c.Param("id")).Count(&count)
	if count > 0 {
		db.Debug().Table("content").Where("id=? and deleted_by_user_id is null", c.Param("id")).Update(UpdateStatus{Status: c.Param("status"), ModifiedAt: time.Now()})
		// var seasoncount int
		// db.Table("season s").Where("s.content_id = ? and s.deleted_by_user_id is null", c.Param("id")).Count(&seasoncount)
		// if seasoncount > 0 {
		// 	db.Table("season s").Where("s.content_id =? and s.deleted_by_user_id is null", c.Param("id")).Update(UpdateStatus{Status: c.Param("status"), ModifiedAt: time.Now()})
		// }
		res := map[string]string{
			"id": c.Param("id"),
		}
		db.Debug().Raw("select content_type from content where id=?", c.Param("id")).Find(&contenttype)
		go common.CreateRedisKeyForContentType(contenttype.ContentType, c)
		c.JSON(http.StatusOK, gin.H{"data": res})
		return
	} else {
		errorFlag = true
		invalidError = common.InvalidError{Code: "error_content_not_found", Description: "The specified condition was not met for 'Id'."}

		invalid.Id = invalidError

		finalErrorResponse = common.FinalErrorResponse{Error: "not_found", Description: "Not found.", Code: "", RequestId: randstr.String(32), Invalid: invalid}

	}
	if errorFlag {
		c.JSON(http.StatusBadRequest, finalErrorResponse)
		return
	}
}

// For Delete one tier or multitier details by one tier id or multitier id-Delete Season or episode details by season id or episode id
// DELETE /api/contents/{id}
// @Summary Delete one tier or multitier details by one tier id or multitier id
// @Description delete Delete one tier or multitier details by one tier id or multitier id
// @Tags onetier or multitier
// @Accept  json
// @Produce  json
// @security Authorization
// @Param id path string true "Id"
// @Success 200 {array} object c.JSON
// @Router /api/contents/{value}/{id} [delete]
func (hs *HandlerService) DeleteOnetierorMultitier(c *gin.Context) {
	db := c.MustGet("DB").(*gorm.DB)
	var errorFlag bool
	errorFlag = false
	var invalidError common.InvalidError
	var invalid common.Invalids
	var finalErrorResponse common.FinalErrorResponse
	var contenttype ContentType
	fmt.Println("Delete of the content", c.MustGet("userid"), c.MustGet("AuthorizationRequired"), c.MustGet("is_back_office_user"))
	if c.MustGet("AuthorizationRequired") == 1 || c.MustGet("is_back_office_user") == false {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Authorization has been denied for this request."})
		return
	}
	userId := c.MustGet("userid")
	if userId.(string) == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Authorization has been denied for this request."})
		return
	}
	var count int
	db.Debug().Table("content").Where("id=? and deleted_by_user_id is null", c.Param("result")).Count(&count)
	if count > 0 {
		db.Debug().Table("content").Where("id=? and  deleted_by_user_id is null", c.Param("result")).Update(UpdateDetails{DeletedByUserId: userId.(string), ModifiedAt: time.Now()})
		db.Debug().Table("season").Where("content_id = ? and deleted_by_user_id is null", c.Param("result")).Update(UpdateDetails{DeletedByUserId: userId.(string), ModifiedAt: time.Now()})
		db.Debug().Raw("select content_type from content where id=?", c.Param("result")).Find(&contenttype)
		go common.CreateRedisKeyForContentType(contenttype.ContentType, c)
		return
	} else {
		errorFlag = true
		invalidError = common.InvalidError{Code: "error_content_not_found", Description: "The specified condition was not met for 'Id'."}
		invalid.Id = invalidError
		finalErrorResponse = common.FinalErrorResponse{Error: "not_found", Description: "Not found.", Code: "", RequestId: randstr.String(32), Invalid: invalid}
	}
	if errorFlag {
		c.JSON(http.StatusBadRequest, finalErrorResponse)
		return
	}
}

// For Create Or Update published Multitier Content Details-Create Or Update published Multitier Content Detail
// POST /api/contents/multitier/published
// @Summary Create Or Update published Multitier Content Detail
// @Description  Create Or Update published Multitier Content Detail
// @Tags  multitier
// @Accept  json
// @Produce  json
// @security Authorization
// @Param id path string true "Id"
// @Param result path string true "published or draft"
// @Param body body MainResponse true "Raw JSON string"
// @Success 200 {array} object c.JSON
// @Router /api/contents/multitier/published [post]
func (hs *HandlerService) CreateOrUpdatePublishedMultitierContentDetails(c *gin.Context) {
	db := c.MustGet("DB").(*gorm.DB)
	var mainResponse MainResponse
	if c.MustGet("AuthorizationRequired") == 1 || c.MustGet("is_back_office_user") == false {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Authorization has been denied for this request."})
		return
	}
	if err := c.ShouldBindJSON(&mainResponse); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error(), "status": http.StatusBadRequest})
		return
	}
	userid := c.MustGet("userid")
	//	var statusdetails StatusDetails
	var serverError = common.ServerErrorResponse()
	primaryinforesponse := mainResponse.PrimaryInfo
	primaryupdate := PrimaryInfoRequest{OriginalTitle: primaryinforesponse.OriginalTitle, AlternativeTitle: primaryinforesponse.AlternativeTitle, ArabicTitle: primaryinforesponse.ArabicTitle, TransliteratedTitle: primaryinforesponse.TransliteratedTitle, Notes: primaryinforesponse.Notes, IntroStart: primaryinforesponse.IntroStart, OutroStart: primaryinforesponse.OutroStart}

	seoresponse := mainResponse.SeoDetails
	updateresponse := SeoDetailsResponse{ContentType: primaryinforesponse.ContentType, EnglishMetaTitle: seoresponse.EnglishMetaTitle, ArabicMetaTitle: seoresponse.ArabicMetaTitle, EnglishMetaDescription: seoresponse.EnglishMetaDescription, ArabicMetaDescription: seoresponse.ArabicMetaDescription, ModifiedAt: time.Now(), Status: 1}

	var primaryInfoIdDetails PrimaryInfoIdDetails
	var contenttype ContentType
	// var primaryDetails PrimaryInfo
	// var errorFlag bool
	// errorFlag = false
	// var arabicTitleError common.ArabicTitleError
	// var transliratederr common.EnglishTitleError
	// var invalid common.Invalids
	// if c.Param("id") != "" {
	// 	db.Table("content_primary_info").Select("transliterated_title,arabic_title").Where("alternative_title=? or arabic_title=? or transliterated_title=? and id!=(select primary_info_id from content where id=?)", primaryinforesponse.AlternativeTitle, primaryinforesponse.ArabicTitle, primaryinforesponse.TransliteratedTitle, c.Param("id")).Find(&primaryDetails)
	// 	if primaryDetails.ArabicTitle == primaryinforesponse.ArabicTitle {
	// 		errorFlag = true
	// 		arabicTitleError = common.ArabicTitleError{Code: "error_arabic_title_not_unique", Description: "title with specified arabic title  " + primaryinforesponse.ArabicTitle + " already exists "}
	// 	}
	// 	if primaryDetails.TransliteratedTitle == primaryinforesponse.TransliteratedTitle {
	// 		errorFlag = true
	// 		transliratederr = common.EnglishTitleError{Code: "error_transilerated_title_not_unique", Description: "title with specified transilerated_title  " + primaryinforesponse.TransliteratedTitle + " already exists "}
	// 	}
	// } else {
	// 	db.Table("content_primary_info").Select("transliterated_title,arabic_title").Where("alternative_title=? or arabic_title=? or transliterated_title=?", primaryinforesponse.AlternativeTitle, primaryinforesponse.ArabicTitle, primaryinforesponse.TransliteratedTitle).Find(&primaryDetails)
	// 	if primaryDetails.ArabicTitle == primaryinforesponse.ArabicTitle {
	// 		errorFlag = true
	// 		arabicTitleError = common.ArabicTitleError{Code: "error_arabic_title_not_unique", Description: "title with specified arabic title  " + primaryinforesponse.ArabicTitle + " already exists "}
	// 	}
	// 	if primaryDetails.TransliteratedTitle == primaryinforesponse.TransliteratedTitle {
	// 		errorFlag = true
	// 		transliratederr = common.EnglishTitleError{Code: "error_transilerated_title_not_unique", Description: "title with specified transilerated_title  " + primaryinforesponse.TransliteratedTitle + " already exists "}
	// 	}
	// }
	// if arabicTitleError.Code != "" {
	// 	invalid.ArabicTitleError = arabicTitleError
	// }
	// if transliratederr.Code != "" {
	// 	invalid.EnglishTitleError = transliratederr
	// }
	// var finalErrorResponse common.FinalErrorResponse
	// finalErrorResponse = common.FinalErrorResponse{Error: "invalid_request", Description: "Validation failed.", Code: "error_validation_failed", RequestId: randstr.String(32), Invalid: invalid}
	// if errorFlag {
	// 	c.JSON(http.StatusBadRequest, finalErrorResponse)
	// 	return
	// }

	// result := db.Table("content").Select("id,status").Where("id=?", c.Param("id"))
	// result.Scan(&statusdetails)
	db.Debug().Raw("select content_type from content where id=?", c.Param("id")).Find(&contenttype)
	// update multitier
	if c.Param("id") != "" {
		if primaryinfoid := db.Debug().Table("content").Select("primary_info_id").Where("id=?", c.Param("id")).Find(&primaryInfoIdDetails).Error; primaryinfoid != nil {
			c.JSON(http.StatusInternalServerError, serverError)
			return
		}
		if primaryinfoupdate := db.Debug().Table("content_primary_info").Where("id=?", primaryInfoIdDetails.PrimaryInfoId).Update(primaryupdate).Error; primaryinfoupdate != nil {
			c.JSON(http.StatusInternalServerError, serverError)
			return
		}

		if contentupdate := db.Debug().Table("content").Where("id=?", c.Param("id")).Update(updateresponse).Error; contentupdate != nil {
			c.JSON(http.StatusInternalServerError, serverError)
			return
		}
		db.Debug().Table("content_genre").Where("content_id=? ", c.Param("id")).Delete(&ContentGenres{})

		for i, data := range mainResponse.ContentGenres {
			fmt.Println(data, "genree data22")

			contentGenre := ContentGenre{ContentId: c.Param("id"), Order: i + 1, GenreId: data.GenreId}
			if genreupdate := db.Debug().Table("content_genre").Where("content_id=? and id=?", c.Param("id"), data.Id).Create(&contentGenre).Error; genreupdate != nil {
				c.JSON(http.StatusInternalServerError, serverError)
				return
			}
			db.Debug().Table("content_subgenre").Where("content_genre_id=?", data.Id).Delete(&ContentSubgenre{})

			for i, value := range data.SubgenreId {

				contentSubGenre := ContentSubgenre{ContentGenreId: contentGenre.Id, Order: i + 1, SubgenreId: value}
				if subgenreupdate := db.Debug().Table("content_subgenre").Where("content_genre_id=?", contentGenre.Id).Create(&contentSubGenre).Error; subgenreupdate != nil {
					c.JSON(http.StatusInternalServerError, serverError)
				}
			}
		}
		res := map[string]string{
			"id": c.Param("id"),
		}
		var contentkey common.ContentKey
		db.Debug().Raw("select content_key from content where id=?", c.Param("id")).Find(&contentkey)
		/* Prepare Redis Cache for single content*/
		contentkeyconverted := strconv.Itoa(contentkey.ContentKey)
		go common.CreateRedisKeyForContent(contentkeyconverted, c)
		/* Prepare Redis Cache for all contents*/
		go common.CreateRedisKeyForContentTypeMTC(c)
		c.JSON(http.StatusOK, gin.H{"data": res})
		return
	} else {
		// create multitier
		var seoDetailsResponse SeoDetailsResponse
		var contentKeyResponse ContentKeyResponse
		if contentkeyresult := db.Table("content").Select("max(content_key) as content_key").Find(&contentKeyResponse).Error; contentkeyresult != nil {
			c.JSON(http.StatusInternalServerError, serverError)
		}
		seoDetailsResponse.Status = 1
		contentkey := mainResponse.ContentKey
		// contentkey := contentKeyResponse.ContentKey + 1
		seoDetailsResponse.ContentType = primaryinforesponse.ContentType
		seoDetailsResponse.ContentKey = contentkey
		seoDetailsResponse.ContentTier = 2
		seoDetailsResponse.CreatedAt = time.Now()
		seoDetailsResponse.ModifiedAt = time.Now()
		seoDetailsResponse.EnglishMetaTitle = seoresponse.EnglishMetaTitle
		seoDetailsResponse.ArabicMetaTitle = seoresponse.ArabicMetaTitle
		seoDetailsResponse.EnglishMetaDescription = seoresponse.EnglishMetaDescription
		seoDetailsResponse.ArabicMetaDescription = seoresponse.ArabicMetaDescription
		seoDetailsResponse.CreatedByUserId = userid.(string)
		var contentmusic ContentMusic
		if err := db.Debug().Table("content_music").Create(&contentmusic).Error; err != nil {
			c.JSON(http.StatusInternalServerError, serverError)
			return
		}
		var contenttaginfo ContentTagInfo
		if err := db.Debug().Table("content_tag_info").Create(&contenttaginfo).Error; err != nil {
			c.JSON(http.StatusInternalServerError, serverError)
			return
		}
		var contentcast ContentCasts
		contentcast.MainActorId = nil
		contentcast.MainActressId = nil
		if err := db.Debug().Table("content_cast").Create(&contentcast).Error; err != nil {
			c.JSON(http.StatusInternalServerError, serverError)
			return
		}

		// for creating old contents with .net take content id request body
		seoDetailsResponse.Id = mainResponse.ContentId
		seoDetailsResponse.CastId = contentcast.Id
		seoDetailsResponse.MusicId = contentmusic.Id
		seoDetailsResponse.TagInfoId = contenttaginfo.Id
		fmt.Println("create content values publish: ", seoDetailsResponse, seoDetailsResponse.ContentType, primaryinforesponse.ContentType, seoDetailsResponse.ContentKey, mainResponse.ContentKey)
		if contentupdate := db.Debug().Table("content").Create(&seoDetailsResponse).Error; contentupdate != nil {
			c.JSON(http.StatusInternalServerError, serverError)
			return
		}
		if primaryinfoupdate := db.Debug().Table("content_primary_info").Create(&primaryupdate).Error; primaryinfoupdate != nil {
			c.JSON(http.StatusInternalServerError, serverError)
			return
		}
		if contentpriamryinfoupdate := db.Debug().Table("content").Where("id=?", seoDetailsResponse.Id).Update("primary_info_id", primaryupdate.Id).Error; contentpriamryinfoupdate != nil {
			c.JSON(http.StatusInternalServerError, serverError)
			return
		}

		for i, data := range mainResponse.ContentGenres {

			contentresponse := ContentGenreResponse{ContentId: seoDetailsResponse.Id, Order: i + 1, GenreId: data.GenreId}
			if genreupdate := db.Debug().Table("content_genre").Create(&contentresponse).Error; genreupdate != nil {
				c.JSON(http.StatusInternalServerError, serverError)
				return
			}

			for i, value := range data.SubgenreId {
				subgenreresponse := SubGenreResponse{ContentGenreId: contentresponse.Id, Order: i + 1, SubgenreId: value}
				if subgenreupdate := db.Debug().Table("content_subgenre").Create(&subgenreresponse).Error; subgenreupdate != nil {
					c.JSON(http.StatusInternalServerError, serverError)
				}
			}
		}
		res := map[string]string{
			"id": seoDetailsResponse.Id,
		}
		/* Prepare Redis Cache for all contents*/
		go common.CreateRedisKeyForContentType(contenttype.ContentType, c)
		c.JSON(http.StatusOK, gin.H{"data": res})
		return
	}

}

// For Create Or Update Draft Multitier Content Details-Create Or Update Draft Multitier Content Details
// POST /api/contents/multitier/:result/:id
// @Summary  Create Or Update Draft Multitier Content Details
// @Description  Create Or Update Draft Multitier Content Details
// @Tags  multitier
// @Accept  json
// @Produce  json
// @security Authorization
// @Param id path string true "Id"
// @Param result path string true "published or draft"
// @Param body body MainResponse true "Raw JSON string"
// @Success 200 {array} object c.JSON
// @Router /api/contents/multitier/{result}/{id} [post]
func (hs *HandlerService) CreateOrUpdateDraftMultitierContentDetails(c *gin.Context) {
	db := c.MustGet("DB").(*gorm.DB)
	if c.MustGet("AuthorizationRequired") == 1 || c.MustGet("is_back_office_user") == false {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Authorization has been denied for this request."})
		return
	}
	userid := c.MustGet("userid")
	var mainResponse MainResponse
	var errorresponse = common.ServerErrorResponse()
	if err := c.ShouldBindJSON(&mainResponse); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error(), "status": http.StatusBadRequest})
		return
	}

	//	var statusdetails StatusDetails

	primaryinforesponse := mainResponse.PrimaryInfo
	primaryupdate := PrimaryInfoRequest{OriginalTitle: primaryinforesponse.OriginalTitle, AlternativeTitle: primaryinforesponse.AlternativeTitle, ArabicTitle: primaryinforesponse.ArabicTitle, TransliteratedTitle: primaryinforesponse.TransliteratedTitle, Notes: primaryinforesponse.Notes, IntroStart: primaryinforesponse.IntroStart, OutroStart: primaryinforesponse.OutroStart}

	seoresponse := mainResponse.SeoDetails
	updateresponse := SeoDetailsResponse{ContentType: primaryinforesponse.ContentType, EnglishMetaTitle: seoresponse.EnglishMetaTitle, ArabicMetaTitle: seoresponse.ArabicMetaTitle, EnglishMetaDescription: seoresponse.EnglishMetaDescription, ArabicMetaDescription: seoresponse.ArabicMetaDescription, ModifiedAt: time.Now(), Status: 3}

	var primaryInfoIdDetails PrimaryInfoIdDetails

	// result := db.Table("content").Select("id,status").Where("id=?", c.Param("id"))
	// result.Scan(&statusdetails)
	// var primaryDetails PrimaryInfo
	// var errorFlag bool
	// errorFlag = false
	// var arabicTitleError common.ArabicTitleError
	// var transliratederr common.EnglishTitleError
	// var invalid common.Invalids
	// if c.Param("id") != "" {
	// 	db.Table("content_primary_info").Select("transliterated_title,arabic_title").Where("alternative_title=? or arabic_title=? or transliterated_title=? and id!=(select primary_info_id from content where id=?)", primaryinforesponse.AlternativeTitle, primaryinforesponse.ArabicTitle, primaryinforesponse.TransliteratedTitle, c.Param("id")).Find(&primaryDetails)
	// 	if primaryDetails.ArabicTitle == primaryinforesponse.ArabicTitle {
	// 		errorFlag = true
	// 		arabicTitleError = common.ArabicTitleError{Code: "error_arabic_title_not_unique", Description: "title with specified arabic title  " + primaryinforesponse.ArabicTitle + " already exists "}
	// 	}
	// 	if primaryDetails.TransliteratedTitle == primaryinforesponse.TransliteratedTitle {
	// 		errorFlag = true
	// 		transliratederr = common.EnglishTitleError{Code: "error_transilerated_title_not_unique", Description: "title with specified transilerated_title  " + primaryinforesponse.TransliteratedTitle + " already exists "}
	// 	}
	// } else {
	// 	db.Debug().Table("content_primary_info").Select("transliterated_title,arabic_title").Where("alternative_title=? or arabic_title=? or transliterated_title=?", primaryinforesponse.AlternativeTitle, primaryinforesponse.ArabicTitle, primaryinforesponse.TransliteratedTitle).Find(&primaryDetails)
	// 	if primaryDetails.ArabicTitle == primaryinforesponse.ArabicTitle {
	// 		errorFlag = true
	// 		arabicTitleError = common.ArabicTitleError{Code: "error_arabic_title_not_unique", Description: "title with specified arabic title  " + primaryinforesponse.ArabicTitle + " already exists "}
	// 	}
	// 	if primaryDetails.TransliteratedTitle == primaryinforesponse.TransliteratedTitle {
	// 		errorFlag = true
	// 		transliratederr = common.EnglishTitleError{Code: "error_transilerated_title_not_unique", Description: "title with specified transilerated_title  " + primaryinforesponse.TransliteratedTitle + " already exists "}
	// 	}
	// }
	// if arabicTitleError.Code != "" {
	// 	invalid.ArabicTitleError = arabicTitleError
	// }
	// if transliratederr.Code != "" {
	// 	invalid.EnglishTitleError = transliratederr
	// }
	// var finalErrorResponse common.FinalErrorResponse
	// finalErrorResponse = common.FinalErrorResponse{Error: "invalid_request", Description: "Validation failed.", Code: "error_validation_failed", RequestId: randstr.String(32), Invalid: invalid}
	// if errorFlag {
	// 	c.JSON(http.StatusBadRequest, finalErrorResponse)
	// 	return
	// }

	// update multiter
	if c.Param("id") != "" {
		if primaryinfoid := db.Debug().Table("content").Select("primary_info_id").Where("id=?", c.Param("id")).Find(&primaryInfoIdDetails).Error; primaryinfoid != nil {
			c.JSON(http.StatusInternalServerError, errorresponse)
			return
		}
		if primaryinfoupdate := db.Debug().Table("content_primary_info").Where("id=?", primaryInfoIdDetails.PrimaryInfoId).Update(primaryupdate).Error; primaryinfoupdate != nil {
			c.JSON(http.StatusInternalServerError, errorresponse)
			return
		}

		if contentupdate := db.Debug().Table("content").Where("id=?", c.Param("id")).Update(updateresponse).Error; contentupdate != nil {
			c.JSON(http.StatusInternalServerError, errorresponse)
			return
		}
		db.Debug().Table("content_genre").Where("content_id=?", c.Param("id")).Delete(&ContentGenres{})

		for i, data := range mainResponse.ContentGenres {
			fmt.Println(data, "genree dat---a")

			contentGenre := ContentGenre{ContentId: c.Param("id"), Order: i + 1, GenreId: data.GenreId}
			if genreupdate := db.Debug().Table("content_genre").Where("content_id=? and id=?", c.Param("id"), data.Id).Create(&contentGenre).Error; genreupdate != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": genreupdate.Error(), "status": http.StatusInternalServerError})
				return
			}
			db.Debug().Table("content_subgenre").Where("content_genre_id=? ", data.Id).Delete(&ContentSubgenre{})
			for i, value := range data.SubgenreId {
				contentSubGenre := ContentSubgenre{ContentGenreId: contentGenre.Id, Order: i + 1, SubgenreId: value}
				if subgenreupdate := db.Debug().Table("content_subgenre").Where("content_genre_id=?", contentGenre.Id).Create(&contentSubGenre).Error; subgenreupdate != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"message": subgenreupdate.Error(), "status": http.StatusInternalServerError})
				}
			}
		}
		res := map[string]string{
			"id": c.Param("id"),
		}
		go common.CreateRedisKeyForContentTypeMTC(c)
		c.JSON(http.StatusOK, gin.H{"data": res})
		return
	} else {
		// create multitier
		var seoDetailsResponse SeoDetailsResponse
		seoDetailsResponse.Status = 3
		var contentKeyResponse ContentKeyResponse
		if contentkeyresult := db.Debug().Table("content").Select("max(content_key) as content_key").Find(&contentKeyResponse).Error; contentkeyresult != nil {
			c.JSON(http.StatusInternalServerError, errorresponse)
			return
		}
		contentkey := mainResponse.ContentKey
		// contentkey := contentKeyResponse.ContentKey + 1
		// for creating old contents with .net take content id and createdby userid from request body
		seoDetailsResponse.Id = mainResponse.ContentId
		//	seoDetailsResponse.CreatedByUserId = mainResponse.CreatedByUserId

		seoDetailsResponse.ContentType = primaryinforesponse.ContentType
		seoDetailsResponse.ContentKey = contentkey
		seoDetailsResponse.ContentTier = 2
		seoDetailsResponse.EnglishMetaTitle = seoresponse.EnglishMetaTitle
		seoDetailsResponse.ArabicMetaTitle = seoresponse.ArabicMetaTitle
		seoDetailsResponse.EnglishMetaDescription = seoresponse.EnglishMetaDescription
		seoDetailsResponse.ArabicMetaDescription = seoresponse.ArabicMetaDescription
		seoDetailsResponse.CreatedByUserId = userid.(string)
		seoDetailsResponse.CreatedAt = time.Now()
		seoDetailsResponse.ModifiedAt = time.Now()
		var contentmusic ContentMusic
		if err := db.Debug().Table("content_music").Create(&contentmusic).Error; err != nil {
			c.JSON(http.StatusInternalServerError, errorresponse)
			return
		}
		var contenttaginfo ContentTagInfo
		if err := db.Debug().Table("content_tag_info").Create(&contenttaginfo).Error; err != nil {
			c.JSON(http.StatusInternalServerError, errorresponse)
			return
		}
		var contentcast ContentCasts
		contentcast.MainActorId = nil
		contentcast.MainActressId = nil
		if err := db.Debug().Table("content_cast").Create(&contentcast).Error; err != nil {
			c.JSON(http.StatusInternalServerError, errorresponse)
			return
		}
		seoDetailsResponse.CastId = contentcast.Id
		seoDetailsResponse.MusicId = contentmusic.Id
		seoDetailsResponse.TagInfoId = contenttaginfo.Id
		fmt.Println("create content values: ", seoDetailsResponse, seoDetailsResponse.ContentType, primaryinforesponse.ContentType, seoDetailsResponse.ContentKey, mainResponse.ContentKey)
		if contentupdate := db.Debug().Table("content").Create(&seoDetailsResponse).Error; contentupdate != nil {
			c.JSON(http.StatusInternalServerError, errorresponse)
			return
		}
		fmt.Println("create content primary values: ", primaryupdate)
		if primaryinfoupdate := db.Debug().Table("content_primary_info").Create(&primaryupdate).Error; primaryinfoupdate != nil {
			c.JSON(http.StatusInternalServerError, errorresponse)
			return
		}
		if contentpriamryinfoupdate := db.Debug().Table("content").Where("id=?", seoDetailsResponse.Id).Update("primary_info_id", primaryupdate.Id).Error; contentpriamryinfoupdate != nil {
			c.JSON(http.StatusInternalServerError, errorresponse)
			return
		}

		for i, data := range mainResponse.ContentGenres {

			contentresponse := ContentGenreResponse{ContentId: seoDetailsResponse.Id, Order: i + 1, GenreId: data.GenreId}
			if genreupdate := db.Debug().Table("content_genre").Create(&contentresponse).Error; genreupdate != nil {
				c.JSON(http.StatusInternalServerError, errorresponse)
				return
			}

			for i, value := range data.SubgenreId {
				subgenreresponse := SubGenreResponse{ContentGenreId: contentresponse.Id, Order: i + 1, SubgenreId: value}
				if subgenreupdate := db.Debug().Table("content_subgenre").Create(&subgenreresponse).Error; subgenreupdate != nil {
					c.JSON(http.StatusInternalServerError, errorresponse)
					return
				}
			}
		}

		res := map[string]string{
			"id": seoDetailsResponse.Id,
		}
		c.JSON(http.StatusOK, gin.H{"data": res})
		return
	}

}

// For Create Or Update onetier Content Details-Create Or Update onetier Content Details
// POST /api/contents/onetier/published/:id
// @Summary Create Or Update onetier Content Details
// @Description  Create Or Update onetier Content Details
// @Tags onetier or multitier
// @Accept  json
// @Produce  json
// @security Authorization
// @Param id path string true "Id"
// @Param result path string true "published or draft"
// @Param body body OnetierContentRequest true "Raw JSON string"
// @Success 200 {array} object c.JSON
// @Router /api/contents/onetier/published/{id} [post]
func (hs *HandlerService) CreateOrUpdatePublishedOnetierContentDetails(c *gin.Context) {
	if c.MustGet("AuthorizationRequired") == 1 || c.MustGet("is_back_office_user") == false {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Authorization has been denied for this request."})
		return
	}
	var request OnetierContentRequest
	// var req OnetierContentRequestValidtion
	// decoder := json.NewDecoder(c.Request.Body)
	// decoder.Decode(&req)
	userid := c.MustGet("userid")
	c.ShouldBindJSON(&request)
	db := c.MustGet("DB").(*gorm.DB)
	var errorFlag bool
	errorFlag = false
	var primaryInfoError common.PrimaryInfoError
	var contentTypeError common.ContentTypeError
	// var arabicTitleError common.ArabicTitleError
	// var transliratederr common.EnglishTitleError
	// var contentTitle ContentPrimaryInfo
	// if c.Param("id") == "" {
	// 	db.Debug().Table("content_primary_info").Select("transliterated_title,arabic_title").Where("transliterated_title=? or arabic_title=?", request.TextualData.PrimaryInfo.TransliteratedTitle, request.TextualData.PrimaryInfo.ArabicTitle).Find(&contentTitle)
	// 	if contentTitle.ArabicTitle == request.TextualData.PrimaryInfo.ArabicTitle {
	// 		errorFlag = true
	// 		arabicTitleError = common.ArabicTitleError{Code: "error_arabic_title_not_unique", Description: "title with specified arabic title  " + request.TextualData.PrimaryInfo.ArabicTitle + " already exists "}
	// 	}
	// 	if contentTitle.TransliteratedTitle == request.TextualData.PrimaryInfo.TransliteratedTitle {
	// 		errorFlag = true
	// 		transliratederr = common.EnglishTitleError{Code: "error_transilerated_title_not_unique", Description: "title with specified transilerated_title  " + request.TextualData.PrimaryInfo.TransliteratedTitle + " already exists "}
	// 	}
	// } else {
	// 	db.Debug().Table("content_primary_info").Select("transliterated_title,arabic_title").Where("transliterated_title=? and id!=(select primary_info_id from content where id=?) or arabic_title=? and id!=(select primary_info_id from content where id=?)", request.TextualData.PrimaryInfo.TransliteratedTitle, c.Param("id"), request.TextualData.PrimaryInfo.ArabicTitle, c.Param("id")).Find(&contentTitle)
	// 	if contentTitle.ArabicTitle == request.TextualData.PrimaryInfo.ArabicTitle {
	// 		errorFlag = true
	// 		arabicTitleError = common.ArabicTitleError{Code: "error_arabic_title_not_unique", Description: "title with specified arabic title  " + request.TextualData.PrimaryInfo.ArabicTitle + " already exists "}
	// 	}
	// 	if contentTitle.TransliteratedTitle == request.TextualData.PrimaryInfo.TransliteratedTitle {
	// 		errorFlag = true
	// 		transliratederr = common.EnglishTitleError{Code: "error_transilerated_title_not_unique", Description: "title with specified transilerated_title  " + request.TextualData.PrimaryInfo.TransliteratedTitle + " already exists "}
	// 	}
	// }
	if request.TextualData.PrimaryInfo != nil {
		if request.TextualData.PrimaryInfo.ContentType == "" {
			errorFlag = true
			contentTypeError = common.ContentTypeError{Code: "NotEmptyValidator", Description: "'Content Type' should not be empty."}
			fmt.Println(contentTypeError, ";;;;;;;;;;;;;;;;;;;;;;;;")
		}
	}
	if request.TextualData.PrimaryInfo == nil {
		errorFlag = true
		primaryInfoError = common.PrimaryInfoError{Code: "NotEmptyValidator", Description: "'Textual Data. Primary Info' should not be empty."}
		fmt.Println(primaryInfoError, ",,,,,,,,,,,,")
	}

	var contentGenresError common.ContentGenresError
	fmt.Println(len(*request.TextualData.ContentGenres), ";;;;;;;;;;;;;;;;;")
	if len(*request.TextualData.ContentGenres) == 0 {
		errorFlag = true
		contentGenresError = common.ContentGenresError{Code: "NotEmptyValidator", Description: "'Textual Data. Content Genres' should not be empty."}
	}
	var contentVarianceError common.ContentVarianceError
	if len(*request.TextualData.ContentVariances) == 0 {
		errorFlag = true
		contentVarianceError = common.ContentVarianceError{Code: "NotEmptyValidator", Description: "'Content Variances' should not be empty."}
	}
	var casterror common.CastError
	if request.TextualData.Cast == nil {
		errorFlag = true
		casterror = common.CastError{Code: "NotEmptyValidator", Description: "'Textual Data. Cast' should not be empty."}
	}
	var musicError common.MusicError
	if request.TextualData.Music == nil {
		errorFlag = true
		musicError = common.MusicError{Code: "NotEmptyValidator", Description: "'Textual Data. Music' should not be empty."}
	}
	var taginfoError common.TaginfoError
	if request.TextualData.TagInfo == nil {
		errorFlag = true
		taginfoError = common.TaginfoError{Code: "NotEmptyValidator", Description: "'Textual Data. Tag Info' should not be empty."}
	}
	var abouterror common.AbouttheContentError
	if request.TextualData.AboutTheContent == nil {
		errorFlag = true
		abouterror = common.AbouttheContentError{Code: "NotEmptyValidator", Description: "'Textual Data. About The Content' should not be empty."}
	}
	var nontextualerrror common.NonTextualDataError
	if request.NonTextualData == nil {
		errorFlag = true
		nontextualerrror = common.NonTextualDataError{Code: "NotNullValidator", Description: "'Non Textual Data' must not be empty."}
	}

	var invalid common.Invalids
	if primaryInfoError.Code != "" {
		fmt.Println(primaryInfoError.Code)
		invalid.PrimaryInfoError = primaryInfoError
		fmt.Println(primaryInfoError, "................................")
	}

	if contentTypeError.Code != "" {
		invalid.ContentTypeError = contentTypeError
	}
	if contentGenresError.Code != "" {
		invalid.ContentGenresError = contentGenresError
		fmt.Println(contentGenresError)
	}
	if contentVarianceError.Code != "" {
		invalid.ContentVarianceError = contentVarianceError
	}
	if casterror.Code != "" {
		invalid.CastError = casterror
	}
	if musicError.Code != "" {
		invalid.MusicError = musicError
	}
	if taginfoError.Code != "" {
		invalid.TaginfoError = taginfoError
	}
	if abouterror.Code != "" {
		invalid.AbouttheContentError = abouterror
	}
	if nontextualerrror.Code != "" {
		invalid.NonTextualDataError = nontextualerrror
	}
	// if arabicTitleError.Code != "" {
	// 	invalid.ArabicTitleError = arabicTitleError
	// }
	// if transliratederr.Code != "" {
	// 	invalid.EnglishTitleError = transliratederr
	// }
	var finalErrorResponse common.FinalErrorResponse
	finalErrorResponse = common.FinalErrorResponse{Error: "invalid_request", Description: "Validation failed.", Code: "error_validation_failed", RequestId: randstr.String(32), Invalid: invalid}
	if errorFlag {
		c.JSON(http.StatusBadRequest, finalErrorResponse)
		return
	}

	errorresponse := common.ServerErrorResponse()
	//	var contentVariance ContentVariance
	var contentRights ContentRights
	var contentTranslation ContentTranslation
	var contentRightsCountry ContentRightsCountry
	var varianceTrailer VarianceTrailer
	var aboutTheContentInfo AboutTheContentInfo
	var contentCast ContentCast
	var playbackItem PlaybackItem
	//var contentGenre ContentGenre
	var playbackItemTargetPlatform PlaybackItemTargetPlatform
	//	var content Content
	var contentRightsPlan ContentRightsPlan
	var rightsProduct RightsProduct
	var productionCountry ProductionCountry
	var seoDetailsResponse Content
	ctx := context.Background()
	tx := db.BeginTx(ctx, nil)
	var statusdetails StatusDetails
	var newarray []int
	for _, data := range *request.TextualData.ContentVariances {
		var ditalarray []int
		for _, data := range data.DigitalRightsRegions {
			ditalarray = append(ditalarray, data)
		}
		newarray = append(newarray, ditalarray...)
	}
	var errorFlags bool
	errorFlags = RemoveDuplicateValues(newarray)
	if errorFlags {
		c.JSON(http.StatusBadRequest, common.ServerError{Error: "countries exists", Description: "Selected countries for this variant are not allowed.", Code: "", RequestId: randstr.String(32)})
		return
	}
	if c.Param("id") != "" {
		tx.Debug().Table("content").Select("id,status,cast_id,music_id,tag_info_id").Where("id=?", c.Param("id")).Find(&statusdetails)
	}
	fmt.Println(statusdetails, "............")
	primaryinforesponse := request.TextualData.PrimaryInfo
	primaryupdate := ContentPrimaryInfo{OriginalTitle: primaryinforesponse.OriginalTitle, AlternativeTitle: primaryinforesponse.AlternativeTitle, ArabicTitle: primaryinforesponse.ArabicTitle, TransliteratedTitle: primaryinforesponse.TransliteratedTitle, Notes: primaryinforesponse.Notes, IntroStart: primaryinforesponse.IntroStart, OutroStart: primaryinforesponse.OutroStart}

	actorsdata := request.TextualData.Cast
	contentCast = ContentCast{MainActorId: actorsdata.MainActorId, MainActressId: actorsdata.MainActressId}
	fmt.Println(contentCast, "content cast")
	if c.Param("id") == "" {
		if res := tx.Debug().Table("content_cast").Create(&contentCast).Error; res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
			return
		}
	} else {
		if res := tx.Debug().Table("content_cast").Where("id=?", statusdetails.CastId).Update(contentCast).Error; res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
			return
		}
	}
	var contentmusic ContentMusic
	if c.Param("id") == "" {
		if res := tx.Debug().Table("content_music").Create(&contentmusic).Error; res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
			return
		}
	}
	// actor
	var contentactorfinal []interface{}
	if len(request.TextualData.Cast.Actors) > 0 {
		for _, actorrange := range request.TextualData.Cast.Actors {

			contentactor := ContentActor{CastId: contentCast.Id, ActorId: actorrange}
			contentactorfinal = append(contentactorfinal, contentactor)
		}
	}
	var actorfinal []interface{}
	if statusdetails.Id == "" {
		if res := gormbulk.BulkInsert(tx, contentactorfinal, common.BULK_INSERT_LIMIT); res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
		}
	} else {
		var contentactordetails ContentActor
		if res := tx.Debug().Table("content_actor").Where("cast_id=?", statusdetails.CastId).Delete(&contentactordetails).Error; res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
		}
		for _, actorrange := range request.TextualData.Cast.Actors {
			contentactorfinal := ContentActor{CastId: statusdetails.CastId, ActorId: actorrange}
			actorfinal = append(actorfinal, contentactorfinal)
		}
	}
	if statusdetails.Id != "" && len(request.TextualData.Cast.Actors) > 0 {
		if res := gormbulk.BulkInsert(tx, actorfinal, common.BULK_INSERT_LIMIT); res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
		}
	}
	// writer
	var contentwriterfinal []interface{}
	if len(request.TextualData.Cast.Writers) > 0 {
		for _, actorrange := range request.TextualData.Cast.Writers {

			contentwriter := ContentWriter{CastId: contentCast.Id, WriterId: actorrange}
			contentwriterfinal = append(contentwriterfinal, contentwriter)
		}
	}
	var writerfinal []interface{}
	if statusdetails.Id == "" {
		if res := gormbulk.BulkInsert(tx, contentwriterfinal, common.BULK_INSERT_LIMIT); res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
		}
	} else {
		var contentactordetails ContentActor
		if res := tx.Debug().Table("content_writer").Where("cast_id=?", statusdetails.CastId).Delete(&contentactordetails).Error; res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
		}
		for _, writerrange := range request.TextualData.Cast.Writers {
			contentwriter := ContentWriter{CastId: statusdetails.CastId, WriterId: writerrange}
			writerfinal = append(writerfinal, contentwriter)
		}
	}
	if statusdetails.Id != "" && len(request.TextualData.Cast.Writers) > 0 {
		if res := gormbulk.BulkInsert(tx, writerfinal, common.BULK_INSERT_LIMIT); res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
		}
	}
	// director
	var contentdirectorfinal []interface{}
	if len(request.TextualData.Cast.Directors) > 0 {
		for _, actorrange := range request.TextualData.Cast.Directors {

			contentwriter := ContentDirector{CastId: contentCast.Id, DirectorId: actorrange}
			contentdirectorfinal = append(contentdirectorfinal, contentwriter)
		}
	}
	var directorfinal []interface{}
	if statusdetails.Id == "" {
		if res := gormbulk.BulkInsert(tx, contentdirectorfinal, common.BULK_INSERT_LIMIT); res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
		}
	} else {
		var contentdirectordetails ContentDirector
		if res := tx.Table("content_director").Where("cast_id=?", statusdetails.CastId).Delete(&contentdirectordetails).Error; res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
		}
		for _, directorrange := range request.TextualData.Cast.Directors {
			contentdirector := ContentDirector{CastId: statusdetails.CastId, DirectorId: directorrange}
			directorfinal = append(directorfinal, contentdirector)
		}
	}
	if statusdetails.Id != "" && len(request.TextualData.Cast.Directors) > 0 {
		if res := gormbulk.BulkInsert(tx, directorfinal, common.BULK_INSERT_LIMIT); res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
		}
	}
	// singer
	var contentsingerrfinal []interface{}
	if len(request.TextualData.Music.Singers) > 0 {
		for _, musicrange := range request.TextualData.Music.Singers {

			contentsinger := ContentSinger{MusicId: contentmusic.Id, SingerId: musicrange}
			contentsingerrfinal = append(contentsingerrfinal, contentsinger)
		}
	}
	var musicfinal []interface{}
	if statusdetails.Id == "" {
		if res := gormbulk.BulkInsert(tx, contentsingerrfinal, common.BULK_INSERT_LIMIT); res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
		}
	} else {
		var contentsingerdetails ContentSinger
		if res := tx.Debug().Table("content_singer").Where("music_id=?", statusdetails.MusicId).Delete(&contentsingerdetails).Error; res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
		}
		for _, musicrange := range request.TextualData.Music.Singers {
			contentsinger := ContentSinger{MusicId: statusdetails.MusicId, SingerId: musicrange}
			musicfinal = append(musicfinal, contentsinger)
		}
	}
	if statusdetails.Id != "" && len(request.TextualData.Music.Singers) > 0 {
		if res := gormbulk.BulkInsert(tx, musicfinal, common.BULK_INSERT_LIMIT); res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
		}
	}
	// music_composer
	var contentmusiccomposerrfinal []interface{}
	if len(request.TextualData.Music.MusicComposers) > 0 {
		for _, musicrange := range request.TextualData.Music.MusicComposers {

			contentmusiccomposer := ContentMusicComposer{MusicId: contentmusic.Id, MusicComposerId: musicrange}
			contentmusiccomposerrfinal = append(contentmusiccomposerrfinal, contentmusiccomposer)
		}
	}
	var musiccomposerfinal []interface{}
	if statusdetails.Id == "" {
		if res := gormbulk.BulkInsert(tx, contentmusiccomposerrfinal, common.BULK_INSERT_LIMIT); res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
		}
	} else {
		var contentmusiccomposerdetails ContentMusicComposer
		if res := tx.Debug().Table("content_music_composer").Where("music_id=?", statusdetails.MusicId).Delete(&contentmusiccomposerdetails).Error; res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
		}
		for _, musicrange := range request.TextualData.Music.MusicComposers {
			contentmusiccomposer := ContentMusicComposer{MusicId: statusdetails.MusicId, MusicComposerId: musicrange}
			musiccomposerfinal = append(musiccomposerfinal, contentmusiccomposer)
		}
	}
	if statusdetails.Id != "" && len(request.TextualData.Music.MusicComposers) > 0 {
		if res := gormbulk.BulkInsert(tx, musiccomposerfinal, common.BULK_INSERT_LIMIT); res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
		}
	}
	// song writer
	var contentsongwriterfinal []interface{}
	if len(request.TextualData.Music.SongWriters) > 0 {
		for _, songrange := range request.TextualData.Music.SongWriters {
			contentsongwriter := ContentSongWriter{MusicId: contentmusic.Id, SongWriterId: songrange}
			contentsongwriterfinal = append(contentsongwriterfinal, contentsongwriter)
		}
	}
	var songfinal []interface{}
	if statusdetails.Id == "" {
		if res := gormbulk.BulkInsert(tx, contentsongwriterfinal, common.BULK_INSERT_LIMIT); res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
		}
	} else {
		var contentsongwriterdetails ContentSongWriter
		if res := tx.Debug().Table("content_song_writer").Where("music_id=?", statusdetails.MusicId).Delete(&contentsongwriterdetails).Error; res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
		}
		for _, songrange := range request.TextualData.Music.SongWriters {
			contentsongwriter := ContentSongWriter{MusicId: statusdetails.MusicId, SongWriterId: songrange}
			songfinal = append(songfinal, contentsongwriter)
		}
	}
	if statusdetails.Id != "" && len(request.TextualData.Music.SongWriters) > 0 {
		if res := gormbulk.BulkInsert(tx, songfinal, common.BULK_INSERT_LIMIT); res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
		}
	}
	seoresponse := request.TextualData.SeoDetails
	updateresponse := SeoDetailsResponse{ContentType: primaryinforesponse.ContentType, EnglishMetaTitle: seoresponse.EnglishMetaTitle, ArabicMetaTitle: seoresponse.ArabicMetaTitle, EnglishMetaDescription: seoresponse.EnglishMetaDescription, ArabicMetaDescription: seoresponse.ArabicMetaDescription, Status: 1, ModifiedAt: time.Now()}
	//	var primaryInfoIdDetails PrimaryInfoIdDetails
	var Variances []Variance
	var variance Variance
	if c.Param("id") == "" {

		var contentKeyResponse ContentKeyResponse
		if contentkeyresult := tx.Debug().Table("content").Select("max(content_key) as content_key").Find(&contentKeyResponse).Error; contentkeyresult != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"message": contentkeyresult.Error(), "status": http.StatusInternalServerError})
		}
		contentkey := request.ContentKey
		// contentkey := contentKeyResponse.ContentKey + 1
		seoDetailsResponse.ContentKey = contentkey
		seoDetailsResponse.ContentType = primaryinforesponse.ContentType
		// for creating old  contents with .net take id and created by userid from request body
		seoDetailsResponse.Id = request.ContentId
		//	seoDetailsResponse.CreatedByUserId = request.CreatedByUserId

		seoDetailsResponse.Status = 1

		if request.NonTextualData.PosterImage != "" {
			seoDetailsResponse.HasPosterImage = true
		} else {
			seoDetailsResponse.HasPosterImage = false
		}
		if request.NonTextualData.DetailsBackground != "" {
			seoDetailsResponse.HasDetailsBackground = true
		} else {
			seoDetailsResponse.HasDetailsBackground = false
		}
		if request.NonTextualData.MobileDetailsBackground != "" {
			seoDetailsResponse.HasMobileDetailsBackground = true
		} else {
			seoDetailsResponse.HasMobileDetailsBackground = false
		}
		seoDetailsResponse.ContentTier = 1
		seoDetailsResponse.Status = 1
		seoDetailsResponse.CreatedAt = time.Now()
		seoDetailsResponse.ModifiedAt = time.Now()
		seoDetailsResponse.EnglishMetaTitle = seoresponse.EnglishMetaTitle
		seoDetailsResponse.ArabicMetaTitle = seoresponse.ArabicMetaTitle
		seoDetailsResponse.EnglishMetaDescription = seoresponse.EnglishMetaDescription
		seoDetailsResponse.ArabicMetaDescription = seoresponse.ArabicMetaDescription
		seoDetailsResponse.PrimaryInfoId = "00000000-0000-0000-0000-000000000000"
		seoDetailsResponse.AboutTheContentInfoId = "00000000-0000-0000-0000-000000000000"
		seoDetailsResponse.CastId = contentCast.Id
		seoDetailsResponse.MusicId = contentmusic.Id
		seoDetailsResponse.TagInfoId = "00000000-0000-0000-0000-000000000000"
		// seoDetailsResponse.DeletedByUserId = nil
		seoDetailsResponse.CreatedByUserId = userid.(string)

		if primaryinfoupdate := tx.Debug().Table("content_primary_info").Create(&primaryupdate).Error; primaryinfoupdate != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"message": primaryinfoupdate.Error(), "status": http.StatusInternalServerError})
			return
		}
		// if contentpriamryinfoupdate := tx.Table("content").Where("id=?", seoDetailsResponse.Id).Update("primary_info_id", primaryupdate.Id).Error; contentpriamryinfoupdate != nil {
		// 	tx.Rollback()
		// 	c.JSON(http.StatusInternalServerError, gin.H{"message": contentpriamryinfoupdate.Error(), "status": http.StatusInternalServerError})
		// 	return
		// }
		if contentupdate := tx.Debug().Table("content").Create(&seoDetailsResponse).Error; contentupdate != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
			return
		}
		// var order int
		// order = 0
		for i, data := range *request.TextualData.ContentGenres {
			//	order = order + 1
			contentresponse := ContentGenreResponse{ContentId: seoDetailsResponse.Id, Order: i + 1, GenreId: data.GenreId}
			if genreupdate := tx.Debug().Table("content_genre").Create(&contentresponse).Error; genreupdate != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"message": genreupdate.Error(), "status": http.StatusInternalServerError})
				return
			}

			for i, value := range data.SubgenresId {
				subgenreresponse := SubGenreResponse{ContentGenreId: contentresponse.Id, Order: i + 1, SubgenreId: value}
				if subgenreupdate := tx.Debug().Table("content_subgenre").Create(&subgenreresponse).Error; subgenreupdate != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, gin.H{"message": subgenreupdate.Error(), "status": http.StatusInternalServerError})
				}
			}
		}

		/*Create Variance for onetier-content */
		for i, data := range *request.TextualData.ContentVariances {
			contentTranslation = ContentTranslation{LanguageType: common.ContentLanguageOriginTypes(data.LanguageType), DubbingLanguage: data.DubbingLanguage, DubbingDialectId: data.DubbingDialectId, SubtitlingLanguage: data.SubtitlingLanguage}
			if res := tx.Debug().Table("content_translation").Create(&contentTranslation).Error; res != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, errorresponse)
				return
			}
			contentRights = ContentRights{DigitalRightsType: data.DigitalRightsType, DigitalRightsStartDate: data.DigitalRightsStartDate, DigitalRightsEndDate: data.DigitalRightsEndDate}
			if contentrightsres := tx.Debug().Table("content_rights").Create(&contentRights).Error; contentrightsres != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, errorresponse)
				return
			}
			var newarr []interface{}
			for _, value := range data.DigitalRightsRegions {

				contentRightsCountry = ContentRightsCountry{ContentRightsId: contentRights.Id, CountryId: value}
				fmt.Println(contentRightsCountry, "content country is")
				newarr = append(newarr, contentRightsCountry)
			}
			if res := gormbulk.BulkInsert(tx, newarr, common.BULK_INSERT_LIMIT); res != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, errorresponse)
			}

			_, _, duration := common.GetVideoDuration(data.VideoContentId)
			if duration == 0 {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "InValid Content VideoId", "description": "Please provide valid Video ContentId", "code": "", "requestId": randstr.String(32)})
				return
			}
			// take created by userid from request body for creating old contents with .net else take user id from generated token
			playbackItem = PlaybackItem{VideoContentId: data.VideoContentId, TranslationId: contentTranslation.Id, Duration: duration, RightsId: contentRights.Id, CreatedByUserId: userid.(string), SchedulingDateTime: data.SchedulingDateTime}
			if res := tx.Debug().Table("playback_item").Create(&playbackItem).Error; res != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, errorresponse)
				return
			}

			var contentVariance ContentVariance

			// for sync add content varince id
			if len(request.VarianceIds) > 0 {
				contentVariance.ID = request.VarianceIds[i]
			}
			contentVariance.Status = 1
			if data.OverlayPosterImage != "" {
				contentVariance.HasOverlayPosterImage = true
			} else {
				contentVariance.HasOverlayPosterImage = false
			}
			if data.DubbingScript != "" {
				contentVariance.HasDubbingScript = true
			} else {
				contentVariance.HasDubbingScript = false
			}
			if data.SubtitlingScript != "" {
				contentVariance.HasSubtitlingScript = true
			} else {
				contentVariance.HasSubtitlingScript = false
			}
			contentVariance.IntroDuration = data.IntroDuration
			contentVariance.IntroStart = data.IntroStart
			contentVariance.ContentId = seoDetailsResponse.Id
			contentVariance.CreatedAt = time.Now()
			contentVariance.ModifiedAt = time.Now()
			//	contentVariance.DeletedByUserId = "00000000-0000-0000-0000-000000000000"
			contentVariance.ContentId = seoDetailsResponse.Id
			if playbackItem.Id != "" {
				contentVariance.PlaybackItemId = playbackItem.Id
			} else {
				contentVariance.PlaybackItemId = "00000000-0000-0000-0000-000000000000"
			}
			contentVariance.Order = i + 1
			fmt.Println(contentVariance.ContentId)
			fmt.Println(contentVariance.PlaybackItemId)
			//	digitalrights = append(digitalrights, contentVariance)

			if res := tx.Debug().Table("content_variance").Create(&contentVariance).Error; res != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, errorresponse)
				return
			}

			if len(data.VarianceTrailers) != 0 {
				for i, a := range data.VarianceTrailers {
					if a.VideoTrailerId != "" {
						_, _, duration := common.GetVideoDuration(a.VideoTrailerId)
						if duration == 0 {
							c.JSON(http.StatusInternalServerError, gin.H{"error": "InValid Content TrailerId", "description": "Please provide valid Video TrailerId", "code": "", "requestId": randstr.String(32)})
							return
						}
						//		varianceTrailer = VarianceTrailer{Order: i + 1, VideoTrailerId: a.VideoTrailerId, EnglishTitle: a.EnglishTitle, ArabicTitle: a.ArabicTitle, Duration: duration, HasTrailerPosterImage: a.HasTrailerPosterImage, ContentVarianceId: contentVariance.ID}
						var varianceTrailer VarianceTrailer
						// for sync
						if len(data.VarianceTrailerIds) > 0 {
							varianceTrailer.Id = data.VarianceTrailerIds[i]
						}
						varianceTrailer.Order = i + 1
						varianceTrailer.VideoTrailerId = a.VideoTrailerId
						varianceTrailer.EnglishTitle = a.EnglishTitle
						varianceTrailer.ArabicTitle = a.ArabicTitle
						varianceTrailer.Duration = duration
						if a.TrailerPosterImage != "" {
							varianceTrailer.HasTrailerPosterImage = true
						} else {
							varianceTrailer.HasTrailerPosterImage = false
						}
						varianceTrailer.ContentVarianceId = contentVariance.ID

						if res := tx.Debug().Table("variance_trailer").Create(&varianceTrailer).Error; res != nil {
							tx.Rollback()
							c.JSON(http.StatusInternalServerError, errorresponse)
							return
						}
						go ContentVarianceTrailerImageUpload(seoDetailsResponse.Id, contentVariance.ID, varianceTrailer.Id, a.TrailerPosterImage)
					}
				}
			}
			if playbackItem.Id != "" {
				var publishplatform []interface{}
				for _, publishrange := range data.PublishingPlatforms {
					playbackItemTargetPlatform = PlaybackItemTargetPlatform{PlaybackItemId: playbackItem.Id, TargetPlatform: publishrange, RightsId: contentRights.Id}
					publishplatform = append(publishplatform, playbackItemTargetPlatform)
				}
				if res := gormbulk.BulkInsert(tx, publishplatform, common.BULK_INSERT_LIMIT); res != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, errorresponse)
					return
				}
			}
			if len(data.SubscriptionPlans) > 0 {
				for _, contentplanrange := range data.SubscriptionPlans {
					contentRightsPlan = ContentRightsPlan{RightsId: contentRights.Id, SubscriptionPlanId: contentplanrange}
					if res := tx.Debug().Table("content_rights_plan").Create(&contentRightsPlan).Error; res != nil {
						tx.Rollback()
						c.JSON(http.StatusInternalServerError, errorresponse)
						return
					}
				}
			}
			if len(data.Products) > 0 {
				for _, productrange := range data.Products {
					rightsProduct = RightsProduct{RightsId: contentRights.Id, ProductName: productrange}
					if res := tx.Debug().Table("rights_product").Create(&rightsProduct).Error; res != nil {
						tx.Rollback()
						c.JSON(http.StatusInternalServerError, errorresponse)
						return
					}
				}
			}
			variance.Id = contentVariance.ID
			variance.OverlayPosterImage = data.OverlayPosterImage
			variance.DubbingScript = data.DubbingScript
			variance.SubtitlingScript = data.SubtitlingScript
			Variances = append(Variances, variance)
		}
		about := request.TextualData.AboutTheContent
		aboutTheContentInfo = AboutTheContentInfo{OriginalLanguage: about.OriginalLanguage, Supplier: about.Supplier, AcquisitionDepartment: about.AcquisitionDepartment, EnglishSynopsis: about.EnglishSynopsis, ArabicSynopsis: about.ArabicSynopsis, ProductionYear: about.ProductionYear, ProductionHouse: about.ProductionHouse, AgeGroup: about.AgeGroup}
		fmt.Println(aboutTheContentInfo, "about the conent info is")
		if res := tx.Debug().Table("about_the_content_info").Create(&aboutTheContentInfo).Error; res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
			return
		}
		fmt.Println(aboutTheContentInfo.Id)
		if len(about.ProductionCountries) > 0 {
			for _, productionrange := range about.ProductionCountries {
				productionCountry = ProductionCountry{AboutTheContentInfoId: aboutTheContentInfo.Id, CountryId: productionrange}
				if res := tx.Debug().Table("production_country").Create(&productionCountry).Error; res != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, errorresponse)
					return
				}
			}
		}
		fmt.Println("........................................................................")
		var contentTagInfo ContentTagInfo
		if res := tx.Debug().Table("content_tag_info").Create(&contentTagInfo).Error; res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
			return
		}
		var contentTag ContentTag
		for _, tagrange := range request.TextualData.TagInfo.Tags {
			contentTag.TagInfoId = contentTagInfo.Id
			contentTag.TextualDataTagId = tagrange
			if res := tx.Debug().Table("content_tag").Create(&contentTag).Error; res != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, errorresponse)
				return
			}
		}
		var contentupdate Content
		contentupdate.AboutTheContentInfoId = aboutTheContentInfo.Id
		contentupdate.PrimaryInfoId = primaryupdate.Id
		contentupdate.TagInfoId = contentTagInfo.Id
		if res := tx.Debug().Table("content").Where("id=?", seoDetailsResponse.Id).Update(contentupdate).Error; res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
			return
		}
		// contentVariances := ContentVariance{ContentId: seoDetailsResponse.Id, PlaybackItemId: playbackItem.Id}
		// if res := tx.Table("content_variance").Where("id=?", contentVariance.ID).Update(contentVariances).Error; res != nil {
		// 	tx.Rollback()
		// 	c.JSON(http.StatusInternalServerError, errorresponse)
		// 	return
		// }
		id := map[string]string{"id": seoDetailsResponse.Id}
		/* upload images to S3 bucket based on content onetier Id*/
		go ContentFileUpload(request, seoDetailsResponse.Id)
		go ContentVarianceImageUpload(Variances, seoDetailsResponse.Id)

		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, errorresponse)
			return
		}
		/* Fragment Creation */
		fragments.CreateContentFragment(seoDetailsResponse.Id, c)
		/* update dirty count if content in content_sync table */
		common.ContentSynching(seoDetailsResponse.Id, c)
		/* update dirty count in page_sync with contentId relation*/
		//  common.PageSyncWithContentId(seoDetailsResponse.Id, c)

		/* Prepare Redis Cache for all contents*/
		// content key need to be changed once sync removed
		contentKey := strconv.Itoa(contentkey)
		go common.CreateRedisKeyForContent(contentKey, c)
		go common.CreateRedisKeyForContentTypeOTC(c)
		c.JSON(http.StatusOK, gin.H{"data": id})
		// update one tier
	} else {
		var primaryInfoIdDetails Content
		if primaryinfoid := tx.Debug().Table("content").Select("primary_info_id,about_the_content_info_id,content_key").Where("id=?", c.Param("id")).Find(&primaryInfoIdDetails).Error; primaryinfoid != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
			return
		}
		if primaryinfoupdate := tx.Debug().Table("content_primary_info").Where("id=?", primaryInfoIdDetails.PrimaryInfoId).Update(primaryupdate).Error; primaryinfoupdate != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
			return
		}

		if contentupdate := tx.Debug().Table("content").Where("id=?", c.Param("id")).Update(updateresponse).Error; contentupdate != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
			return
		}
		var contentgenreid ContentGenre
		if res := tx.Debug().Table("content_genre").Select("id").Where("content_id=?", c.Param("id")).Find(&contentgenreid).Error; res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
			return
		}
		var contentgenre ContentGenre
		tx.Debug().Table("content_genre").Where("content_id=?", c.Param("id")).Delete(&contentgenre)

		var contentsubgenre ContentSubgenre
		tx.Debug().Table("content_subgenre").Where("content_genre_id=?", contentgenreid.Id).Delete(&contentsubgenre)

		for i, data := range *request.TextualData.ContentGenres {
			contentgenrecreate := ContentGenre{ContentId: c.Param("id"), Order: i + 1, GenreId: data.GenreId}
			if genreupdate := tx.Debug().Table("content_genre").Create(&contentgenrecreate).Error; genreupdate != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, errorresponse)
				return
			}

			for i, value := range data.SubgenresId {
				subgenreresponse := SubGenreResponse{ContentGenreId: contentgenrecreate.Id, Order: i + 1, SubgenreId: value}
				if subgenreupdate := tx.Debug().Table("content_subgenre").Create(&subgenreresponse).Error; subgenreupdate != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, errorresponse)
					return
				}
			}
		}
		// deleting not existing variances
		var existingvariances []ContentVariance
		if err := tx.Debug().Table("content_variance").Select("id").Where("content_id=? and deleted_by_user_id is null", c.Param("id")).Find(&existingvariances).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
			return
		}
		var newarrays []string
		var exist bool
		for _, varaincerange := range existingvariances {
			exist = false
			for _, newvariances := range *request.TextualData.ContentVariances {
				if varaincerange.ID == newvariances.Id {
					exist = true
					break
				}

			}
			if !exist {
				newarrays = append(newarrays, varaincerange.ID)
			}
		}
		fmt.Println(newarrays, "-------------------------------")
		for _, variances := range newarrays {
			tx.Debug().Table("content_variance").Where("content_id=? and id=?", c.Param("id"), variances).Update("deleted_by_user_id", userid.(string))
		}
		var varianceoreder int
		varianceoreder = 0
		for i, value := range *request.TextualData.ContentVariances {

			// var count int
			// tx.Table("content_variance").Select("id").Where("content_id=?", c.Param("id")).Count(&count)
			// fmt.Println(count, "count is")
			if value.Id != "" {
				var contentVariance ContentVariance
				contentVariance.Status = 1
				if value.OverlayPosterImage != "" {
					contentVariance.HasOverlayPosterImage = true
				} else {
					contentVariance.HasOverlayPosterImage = false
				}
				if value.DubbingScript != "" {
					contentVariance.HasDubbingScript = true
				} else {
					contentVariance.HasDubbingScript = false
				}
				if value.SubtitlingScript != "" {
					contentVariance.HasSubtitlingScript = true
				} else {
					contentVariance.HasSubtitlingScript = false
				}
				contentVariance.IntroDuration = value.IntroDuration
				contentVariance.IntroStart = value.IntroStart
				contentVariance.ContentId = seoDetailsResponse.Id
				contentVariance.ModifiedAt = time.Now()
				if res := tx.Debug().Table("content_variance").Where("content_id=? and id=?", c.Param("id"), value.Id).Update(contentVariance).Error; res != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, errorresponse)
					return
				}
				var contentvdetails ContentVariance
				if res := tx.Debug().Table("content_variance").Select("playback_item_id,id").Where("content_id=? and id=?", c.Param("id"), value.Id).Find(&contentvdetails).Error; res != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, errorresponse)
					return
				}
				_, _, duration := common.GetVideoDuration(value.VideoContentId)
				if duration == 0 {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "InValid Content VideoId", "description": "Please provide valid Video ContentId ", "code": "", "requestId": randstr.String(32)})
					return
				}
				playbackItemreq := PlaybackItem{
					VideoContentId:     value.VideoContentId,
					SchedulingDateTime: value.SchedulingDateTime,
					Duration:           duration,
				}
				// new implementation
				if value.SchedulingDateTime == nil {
					if res := tx.Debug().Table("playback_item").Select("scheduling_date_time").Where("id=?", contentvdetails.PlaybackItemId).Updates(map[string]interface{}{"scheduling_date_time": gorm.Expr("NULL")}).Error; res != nil {
						tx.Rollback()
						c.JSON(http.StatusInternalServerError, errorresponse)
						return
					}
					if res := tx.Debug().Table("playback_item").Where("id=?", contentvdetails.PlaybackItemId).Update(playbackItemreq).Error; res != nil {
						tx.Rollback()
						c.JSON(http.StatusInternalServerError, errorresponse)
						return
					}

				} else {
					if res := tx.Debug().Table("playback_item").Where("id=?", contentvdetails.PlaybackItemId).Update(playbackItemreq).Error; res != nil {
						tx.Rollback()
						c.JSON(http.StatusInternalServerError, errorresponse)
						return
					}
				}
				var playbackitems PlaybackItem
				if res := tx.Debug().Table("playback_item").Select("rights_id,translation_id").Where("id=?", contentvdetails.PlaybackItemId).Find(&playbackitems).Error; res != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, errorresponse)
					return
				}
				fmt.Println(playbackitems.RightsId, ".....................")
				contentrights := ContentRights{DigitalRightsType: value.DigitalRightsType, DigitalRightsEndDate: value.DigitalRightsEndDate, DigitalRightsStartDate: value.DigitalRightsStartDate}
				if res := tx.Debug().Table("content_rights").Where("id=?", playbackitems.RightsId).Update(contentrights).Error; res != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, errorresponse)
					return
				}
				if res := tx.Table("content_rights_country").Where("content_rights_id=?", playbackitems.RightsId).Delete(&contentRightsCountry).Error; res != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, errorresponse)
					return
				}
				var regions []interface{}
				for _, data := range value.DigitalRightsRegions {

					contentrightcountry := ContentRightsCountry{ContentRightsId: playbackitems.RightsId, CountryId: data}
					regions = append(regions, contentrightcountry)
				}
				if res := gormbulk.BulkInsert(tx, regions, common.BULK_INSERT_LIMIT); res != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, errorresponse)

				}
				if res := tx.Debug().Table("playback_item_target_platform").Where("playback_item_id=?", contentvdetails.PlaybackItemId).Delete(&playbackItem).Error; res != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, errorresponse)
					return
				}
				var publishplatformarr []interface{}
				for _, publishplatformrange := range value.PublishingPlatforms {
					playbackItemTargetPlatforms := PlaybackItemTargetPlatform{PlaybackItemId: contentvdetails.PlaybackItemId, TargetPlatform: publishplatformrange, RightsId: playbackitems.RightsId}
					publishplatformarr = append(publishplatformarr, playbackItemTargetPlatforms)
				}
				if res := gormbulk.BulkInsert(tx.Debug(), publishplatformarr, common.BULK_INSERT_LIMIT); res != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, errorresponse)

				}
				var contenttranslation ContentTranslation
				contenttranslation.LanguageType = common.ContentLanguageOriginTypes(value.LanguageType)
				contenttranslation.DubbingDialectId = value.DubbingDialectId
				contenttranslation.DubbingLanguage = value.DubbingLanguage
				contenttranslation.SubtitlingLanguage = value.SubtitlingLanguage

				if res := tx.Debug().Table("content_translation").Where("id=?", playbackitems.TranslationId).Update(contenttranslation).Error; res != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, errorresponse)
					return
				}

				tx.Debug().Table("content_rights_plan").Where("rights_id=?", playbackitems.RightsId).Delete(&ContentRightsPlan{})
				fmt.Println(value.SubscriptionPlans, "lllllllllllllllllllll")
				if len(value.SubscriptionPlans) > 0 {
					for _, contentplanrange := range value.SubscriptionPlans {
						contentrightsplans := ContentRightsPlan{RightsId: playbackitems.RightsId, SubscriptionPlanId: contentplanrange}
						if res := tx.Debug().Table("content_rights_plan").Create(&contentrightsplans).Error; res != nil {
							tx.Rollback()
							c.JSON(http.StatusInternalServerError, errorresponse)
							return
						}
					}
				}

				var rights RightsProduct

				tx.Debug().Table("rights_product").Where("rights_id=?", playbackitems.RightsId).Delete(&rights)

				if len(value.Products) > 0 {
					for _, productrange := range value.Products {
						rightsprodu := RightsProduct{RightsId: playbackitems.RightsId, ProductName: productrange}
						if err := tx.Debug().Table("rights_product").Create(&rightsprodu).Error; err != nil {
							c.JSON(http.StatusInternalServerError, errorresponse)
							return
						}
					}
				}

				// deleting not extisting trailer
				var existingtrailers []VarianceTrailer
				var newtrailers []string
				fmt.Println(newtrailers, ".................")
				var exists bool
				tx.Debug().Table("variance_trailer").Select("id").Where("content_variance_id=?", value.Id).Find(&existingtrailers)
				if len(value.VarianceTrailers) > 0 {
					for _, varaincetrailers := range existingtrailers {
						fmt.Println(varaincetrailers.Id, "llllllllllllll")
						fmt.Println(existingtrailers, "kkkkkkkkkkkkkkkkk")
						exists = false
						for _, existing := range value.VarianceTrailers {
							if varaincetrailers.Id == existing.Id {
								exists = true
								fmt.Println(exists, ";;;;;;;;;;;;;;;")
								break

							}
						}
						if !exists {
							newtrailers = append(newtrailers, varaincetrailers.Id)
							fmt.Println(newtrailers, "kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk")
						}
					}
					if len(newtrailers) >= 1 {
						for _, new := range newtrailers {
							tx.Debug().Table("variance_trailer").Where("content_variance_id=? and id=?", value.Id, new).Delete(&varianceTrailer)
						}
					}
				} else {
					tx.Debug().Table("variance_trailer").Where("content_variance_id=? ", value.Id).Delete(&varianceTrailer)
				}
				var trailerorder int
				trailerorder = 0
				for i, variancerange := range value.VarianceTrailers {
					// var totalcount int
					// fmt.Println(value.Id, "kkkkkkkkkkkkkkkkkkk")
					// tx.Debug().Table("variance_trailer").Select("id").Where("id=?", variancerange.Id).Count(&totalcount)
					// fmt.Println(totalcount, "total count is")
					if variancerange.Id != "" {

						// variance trailer update
						_, _, duration := common.GetVideoDuration(variancerange.VideoTrailerId)
						if duration == 0 {
							c.JSON(http.StatusInternalServerError, gin.H{"error": "InValid Content TrailerId", "description": "Please provide valid Video TrailerId", "code": "", "requestId": randstr.String(32)})
							return
						}
						varinceupdattes := VarianceTrailer{Order: variancerange.Order, VideoTrailerId: variancerange.VideoTrailerId, EnglishTitle: variancerange.EnglishTitle, ArabicTitle: variancerange.ArabicTitle, Duration: duration, HasTrailerPosterImage: variancerange.HasTrailerPosterImage}
						if res := tx.Debug().Table("variance_trailer").Where("content_variance_id=? and id=?", value.Id, variancerange.Id).Update(varinceupdattes).Error; res != nil {
							c.JSON(http.StatusInternalServerError, errorresponse)
							return
						}
						go ContentVarianceTrailerImageUpload(c.Param("id"), contentvdetails.ID, variancerange.Id, variancerange.TrailerPosterImage)
					} else {
						// create variance trailer
						if variancerange.VideoTrailerId != "" {
							_, _, duration := common.GetVideoDuration(variancerange.VideoTrailerId)
							if duration == 0 {
								c.JSON(http.StatusInternalServerError, gin.H{"error": "InValid Content TrailerId", "description": "Please provide valid Video TrailerId", "code": "", "requestId": randstr.String(32)})
								return
							}
							// for sync
							varianceTrailer.Id = value.VarianceTrailerIds[trailerorder]

							varianceTrailer.Order = i + 1
							varianceTrailer.VideoTrailerId = variancerange.VideoTrailerId
							varianceTrailer.EnglishTitle = variancerange.EnglishTitle
							varianceTrailer.ArabicTitle = variancerange.ArabicTitle
							varianceTrailer.Duration = duration
							if variancerange.TrailerPosterImage != "" {
								varianceTrailer.HasTrailerPosterImage = true
							} else {
								varianceTrailer.HasTrailerPosterImage = false
							}
							varianceTrailer.ContentVarianceId = value.Id
							if res := tx.Debug().Table("variance_trailer").Create(&varianceTrailer).Error; res != nil {
								tx.Rollback()
								c.JSON(http.StatusInternalServerError, errorresponse)
								return
							}
							trailerorder = trailerorder + 1
							go ContentVarianceTrailerImageUpload(c.Param("id"), varianceTrailer.ContentVarianceId, varianceTrailer.Id, variancerange.TrailerPosterImage)
						}
					}
				}
				variance.Id = value.Id
				variance.OverlayPosterImage = value.OverlayPosterImage
				variance.DubbingScript = value.DubbingScript
				variance.SubtitlingScript = value.SubtitlingScript
				Variances = append(Variances, variance)
			} else {
				/*Create Variance for onetier-content */
				contentTranslation = ContentTranslation{LanguageType: common.ContentLanguageOriginTypes(value.LanguageType), DubbingLanguage: value.DubbingLanguage, DubbingDialectId: value.DubbingDialectId, SubtitlingLanguage: value.SubtitlingLanguage}
				if res := tx.Debug().Table("content_translation").Create(&contentTranslation).Error; res != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, errorresponse)
					return
				}
				contentRights = ContentRights{DigitalRightsType: value.DigitalRightsType, DigitalRightsStartDate: value.DigitalRightsStartDate, DigitalRightsEndDate: value.DigitalRightsEndDate}
				if contentrightsres := tx.Debug().Table("content_rights").Create(&contentRights).Error; contentrightsres != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, errorresponse)
					return
				}
				var newarr []interface{}
				for _, value := range value.DigitalRightsRegions {

					contentRightsCountry = ContentRightsCountry{ContentRightsId: contentRights.Id, CountryId: value}
					fmt.Println(contentRightsCountry, "content country is")
					newarr = append(newarr, contentRightsCountry)
				}
				if res := gormbulk.BulkInsert(tx, newarr, common.BULK_INSERT_LIMIT); res != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, errorresponse)
				}

				_, _, duration := common.GetVideoDuration(value.VideoContentId)
				if duration == 0 {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "InValid Content VideoId", "description": "Please provide valid Video ContentId", "code": "", "requestId": randstr.String(32)})
					return
				}
				playbackItem := PlaybackItem{VideoContentId: value.VideoContentId, TranslationId: contentTranslation.Id, Duration: duration, RightsId: contentRights.Id, CreatedByUserId: userid.(string), SchedulingDateTime: value.SchedulingDateTime}
				if res := tx.Debug().Table("playback_item").Create(&playbackItem).Error; res != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, errorresponse)
					return
				}

				var contentVariance ContentVariance
				// for sync
				contentVariance.ID = request.VarianceIds[varianceoreder]

				contentVariance.Status = 1
				if value.OverlayPosterImage != "" {
					contentVariance.HasOverlayPosterImage = true
				} else {
					contentVariance.HasOverlayPosterImage = false
				}
				if value.DubbingScript != "" {
					contentVariance.HasDubbingScript = true
				} else {
					contentVariance.HasDubbingScript = false
				}
				if value.SubtitlingScript != "" {
					contentVariance.HasSubtitlingScript = true
				} else {
					contentVariance.HasSubtitlingScript = false
				}
				contentVariance.IntroDuration = value.IntroDuration
				contentVariance.IntroStart = value.IntroStart
				contentVariance.ContentId = c.Param("id")
				fmt.Println(contentVariance.ContentId, "llllllllllllllllllllll")
				contentVariance.CreatedAt = time.Now()
				contentVariance.ModifiedAt = time.Now()
				//	contentVariance.DeletedByUserId = "00000000-0000-0000-0000-000000000000"
				contentVariance.PlaybackItemId = playbackItem.Id

				contentVariance.Order = i + 1
				fmt.Println(contentVariance.ContentId, ".....................")
				fmt.Println(contentVariance.PlaybackItemId)
				//	digitalrights = append(digitalrights, contentVariance)

				if res := tx.Debug().Table("content_variance").Create(&contentVariance).Error; res != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, errorresponse)
					return
				}
				varianceoreder = varianceoreder + 1
				var trailersorder int
				trailersorder = 0
				if len(value.VarianceTrailers) != 0 {
					for i, a := range value.VarianceTrailers {
						if a.VideoTrailerId != "" {
							_, _, duration := common.GetVideoDuration(a.VideoTrailerId)
							if duration == 0 {
								c.JSON(http.StatusInternalServerError, gin.H{"error": "InValid Content TrailerId", "description": "Please provide valid Video TrailerId", "code": "", "requestId": randstr.String(32)})
								return
							}
							var varianceTrailer VarianceTrailer
							//	varianceTrailer = VarianceTrailer{Order: i + 1, VideoTrailerId: a.VideoTrailerId, EnglishTitle: a.EnglishTitle, ArabicTitle: a.ArabicTitle, Duration: duration, HasTrailerPosterImage: a.HasTrailerPosterImage, ContentVarianceId: contentVariance.ID}
							// for sync
							// for sync
							varianceTrailer.Id = value.VarianceTrailerIds[trailersorder]

							varianceTrailer.Order = i + 1
							varianceTrailer.VideoTrailerId = a.VideoTrailerId
							varianceTrailer.EnglishTitle = a.EnglishTitle
							varianceTrailer.ArabicTitle = a.ArabicTitle
							varianceTrailer.Duration = duration
							if a.TrailerPosterImage != "" {
								varianceTrailer.HasTrailerPosterImage = true
							} else {
								varianceTrailer.HasTrailerPosterImage = false
							}
							varianceTrailer.ContentVarianceId = contentVariance.ID
							if res := tx.Debug().Table("variance_trailer").Create(&varianceTrailer).Error; res != nil {
								tx.Rollback()
								c.JSON(http.StatusInternalServerError, errorresponse)
								return
							}
							trailersorder = trailersorder + 1

							go ContentVarianceTrailerImageUpload(c.Param("id"), varianceTrailer.ContentVarianceId, varianceTrailer.Id, a.TrailerPosterImage)
						}
					}
				}
				if playbackItem.Id != "" {
					var publishplatform []interface{}
					for _, publishrange := range value.PublishingPlatforms {
						playbackItemTargetPlatform = PlaybackItemTargetPlatform{PlaybackItemId: playbackItem.Id, TargetPlatform: publishrange, RightsId: contentRights.Id}
						publishplatform = append(publishplatform, playbackItemTargetPlatform)
					}
					if res := gormbulk.BulkInsert(tx, publishplatform, common.BULK_INSERT_LIMIT); res != nil {
						tx.Rollback()
						c.JSON(http.StatusInternalServerError, errorresponse)
						return
					}
				}
				if len(value.SubscriptionPlans) > 0 {
					for _, contentplanrange := range value.SubscriptionPlans {
						contentRightsPlan = ContentRightsPlan{RightsId: contentRights.Id, SubscriptionPlanId: contentplanrange}
						if res := tx.Debug().Table("content_rights_plan").Create(&contentRightsPlan).Error; res != nil {
							tx.Rollback()
							c.JSON(http.StatusInternalServerError, errorresponse)
							return
						}
					}
				}
				if len(value.Products) > 0 {
					for _, productrange := range value.Products {
						rightsProduct = RightsProduct{RightsId: contentRights.Id, ProductName: productrange}
						if res := tx.Debug().Table("rights_product").Create(&rightsProduct).Error; res != nil {
							tx.Rollback()
							c.JSON(http.StatusInternalServerError, errorresponse)
							return
						}
					}
				}
				variance.Id = contentVariance.ID
				variance.OverlayPosterImage = value.OverlayPosterImage
				variance.DubbingScript = value.DubbingScript
				variance.SubtitlingScript = value.SubtitlingScript
				fmt.Println(variance.Id, "llllllllllll")
				fmt.Println(variance.OverlayPosterImage, "mmmmmmmmmmmmmmmmmmmmmmmmmm")
				Variances = append(Variances, variance)

			}
		}
		fmt.Println(Variances, "nnnnnnnnnnnnnnnnnnnnnnn")
		var aboutthecontentInfo AboutTheContentInfo
		req := request.TextualData.AboutTheContent
		aboutthecontentInfo.OriginalLanguage = req.OriginalLanguage
		aboutthecontentInfo.Supplier = req.Supplier
		aboutthecontentInfo.AcquisitionDepartment = req.AcquisitionDepartment
		aboutthecontentInfo.EnglishSynopsis = req.EnglishSynopsis
		aboutthecontentInfo.ArabicSynopsis = req.ArabicSynopsis
		aboutthecontentInfo.ProductionYear = req.ProductionYear
		aboutthecontentInfo.ProductionHouse = req.ProductionHouse
		aboutthecontentInfo.AgeGroup = req.AgeGroup
		aboutthecontentInfo.IntroDuration = req.IntroDuration
		aboutthecontentInfo.IntroStart = req.IntroStart
		aboutthecontentInfo.OutroDuration = req.OutroDuration
		aboutthecontentInfo.OutroStart = req.OutroStart

		// var contentabout Content
		// if res := tx.Table("content").Select("about_the_content_info_id").Find(&contentabout).Error; res != nil {
		// 	c.JSON(http.StatusInternalServerError, errorresponse)
		// 	return
		// }
		if res := tx.Debug().Table("about_the_content_info").Where("id=?", primaryInfoIdDetails.AboutTheContentInfoId).Update(aboutthecontentInfo).Error; res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
			return
		}

		if res := tx.Debug().Table("production_country").Where("about_the_content_info_id=?", primaryInfoIdDetails.AboutTheContentInfoId).Delete(&ProductionCountry{}).Error; res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
			return
		}

		if len(req.ProductionCountries) > 0 {
			for _, productionrange := range req.ProductionCountries {
				productionCountry = ProductionCountry{AboutTheContentInfoId: primaryInfoIdDetails.AboutTheContentInfoId, CountryId: productionrange}
				if res := tx.Debug().Table("production_country").Create(&productionCountry).Error; res != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, errorresponse)
					return
				}
			}
		}
		var tagfinal []interface{}
		if len(request.TextualData.TagInfo.Tags) > 0 {
			var taginfo TagInfo
			tx.Debug().Table("content_tag").Where("tag_info_id=?", statusdetails.TagInfoId).Delete(&taginfo)
			for _, tagrange := range request.TextualData.TagInfo.Tags {
				contentTagFinal := ContentTag{TagInfoId: statusdetails.TagInfoId, TextualDataTagId: tagrange}
				tagfinal = append(tagfinal, contentTagFinal)
			}
		}
		if len(tagfinal) > 0 {
			if res := gormbulk.BulkInsert(tx, tagfinal, common.BULK_INSERT_LIMIT); res != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, errorresponse)
				return
			}
		}
		id := map[string]string{"id": c.Param("id")}
		/* upload images to S3 bucket based on content onetier Id*/
		go ContentFileUpload(request, c.Param("id"))
		go ContentVarianceImageUpload(Variances, c.Param("id"))

		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, errorresponse)
			return
		}
		/* Fragment Creation */
		go fragments.CreateContentFragment(c.Param("id"), c)
		/* update dirty count if content modified */
		go common.ContentSynching(c.Param("id"), c)
		/* update dirty count in page_sync with contentId relation*/
		// go common.PageSyncWithContentId(c.Param("id"), c)

		/* Prepare Redis Cache for all contents*/
		// redis for 1 tier need to implement in future
		ContentKey := strconv.Itoa(primaryInfoIdDetails.ContentKey)
		go common.CreateRedisKeyForContent(ContentKey, c)
		go common.CreateRedisKeyForContentTypeOTC(c)
		c.JSON(http.StatusOK, gin.H{"data": id})
	}
}

// For Create Or Update onetier Content Details-Create Or Update onetier Content Details
// POST /api/contents/onetier/draft/:id
// @Summary Create Or Update onetier Content Details
// @Description  Create Or Update onetier Content Details
// @Tags onetier or multitier
// @Accept  json
// @Produce  json
// @security Authorization
// @Param id path string true "Id"
// @Param result path string true "published or draft"
// @Param body body OnetierContentRequest true "Raw JSON string"
// @Success 200 {array} object c.JSON
// @Router /api/contents/onetier/draft/{id} [post]
func (hs *HandlerService) CreateOrUpdateDraftOnetierContentDetails(c *gin.Context) {
	if c.MustGet("AuthorizationRequired") == 1 || c.MustGet("is_back_office_user") == false {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Authorization has been denied for this request."})
		return
	}
	var request OnetierContentRequest
	// var req OnetierContentRequestValidtion
	// decoder := json.NewDecoder(c.Request.Body)
	// decoder.Decode(&req)
	userid := c.MustGet("userid")
	c.ShouldBindJSON(&request)
	db := c.MustGet("DB").(*gorm.DB)
	var errorFlag bool
	errorFlag = false
	var primaryInfoError common.PrimaryInfoError
	var contentTypeError common.ContentTypeError
	// var arabicTitleError common.ArabicTitleError
	// var transliratederr common.EnglishTitleError
	// var contentTitle ContentPrimaryInfo
	// if c.Param("id") == "" {
	// 	db.Table("content_primary_info").Select("transliterated_title,arabic_title").Where("alternative_title=? or arabic_title=?", request.TextualData.PrimaryInfo.AlternativeTitle, request.TextualData.PrimaryInfo.ArabicTitle).Find(&contentTitle)
	// 	if contentTitle.ArabicTitle == request.TextualData.PrimaryInfo.ArabicTitle {
	// 		errorFlag = true
	// 		arabicTitleError = common.ArabicTitleError{Code: "error_arabic_title_not_unique", Description: "title with specified arabic title  " + request.TextualData.PrimaryInfo.ArabicTitle + " already exists "}
	// 	}
	// 	if contentTitle.TransliteratedTitle == request.TextualData.PrimaryInfo.TransliteratedTitle {
	// 		errorFlag = true
	// 		transliratederr = common.EnglishTitleError{Code: "error_transilerated_title_not_unique", Description: "title with specified transilerated_title  " + request.TextualData.PrimaryInfo.TransliteratedTitle + " already exists "}
	// 	}
	// } else {
	// 	db.Table("content_primary_info").Select("transliterated_title,arabic_title").Where("(alternative_title=? or arabic_title=? or transliterated_title=?) and id != (select primary_info_id from content where id=?) ", request.TextualData.PrimaryInfo.AlternativeTitle, request.TextualData.PrimaryInfo.ArabicTitle, request.TextualData.PrimaryInfo.TransliteratedTitle, c.Param("id")).Find(&contentTitle)
	// 	if contentTitle.ArabicTitle == request.TextualData.PrimaryInfo.ArabicTitle {
	// 		errorFlag = true
	// 		arabicTitleError = common.ArabicTitleError{Code: "error_arabic_title_not_unique", Description: "title with specified arabic title  " + request.TextualData.PrimaryInfo.ArabicTitle + " already exists "}
	// 	}
	// 	if contentTitle.TransliteratedTitle == request.TextualData.PrimaryInfo.TransliteratedTitle {
	// 		errorFlag = true
	// 		transliratederr = common.EnglishTitleError{Code: "error_transilerated_title_not_unique", Description: "title with specified transilerated_title  " + request.TextualData.PrimaryInfo.TransliteratedTitle + " already exists "}
	// 	}
	// }
	if request.TextualData.PrimaryInfo != nil {
		if request.TextualData.PrimaryInfo.ContentType == "" {
			errorFlag = true
			contentTypeError = common.ContentTypeError{Code: "NotEmptyValidator", Description: "'Content Type' should not be empty."}
			fmt.Println(contentTypeError, ";;;;;;;;;;;;;;;;;;;;;;;;")
		}
	}
	if request.TextualData.PrimaryInfo == nil {
		errorFlag = true
		primaryInfoError = common.PrimaryInfoError{Code: "NotEmptyValidator", Description: "'Textual Data. Primary Info' should not be empty."}
		fmt.Println(primaryInfoError, ",,,,,,,,,,,,")
	}

	var contentGenresError common.ContentGenresError
	fmt.Println(len(*request.TextualData.ContentGenres), ";;;;;;;;;;;;;;;;;")
	if len(*request.TextualData.ContentGenres) == 0 {
		errorFlag = true
		contentGenresError = common.ContentGenresError{Code: "NotEmptyValidator", Description: "'Textual Data. Content Genres' should not be empty."}
	}
	var contentVarianceError common.ContentVarianceError
	if len(*request.TextualData.ContentVariances) == 0 {
		errorFlag = true
		contentVarianceError = common.ContentVarianceError{Code: "NotEmptyValidator", Description: "'Content Variances' should not be empty."}
	}
	var casterror common.CastError
	if request.TextualData.Cast == nil {
		errorFlag = true
		casterror = common.CastError{Code: "NotEmptyValidator", Description: "'Textual Data. Cast' should not be empty."}
	}
	var musicError common.MusicError
	if request.TextualData.Music == nil {
		errorFlag = true
		musicError = common.MusicError{Code: "NotEmptyValidator", Description: "'Textual Data. Music' should not be empty."}
	}
	var taginfoError common.TaginfoError
	if request.TextualData.TagInfo == nil {
		errorFlag = true
		taginfoError = common.TaginfoError{Code: "NotEmptyValidator", Description: "'Textual Data. Tag Info' should not be empty."}
	}
	var abouterror common.AbouttheContentError
	if request.TextualData.AboutTheContent == nil {
		errorFlag = true
		abouterror = common.AbouttheContentError{Code: "NotEmptyValidator", Description: "'Textual Data. About The Content' should not be empty."}
	}
	var nontextualerrror common.NonTextualDataError
	if request.NonTextualData == nil {
		errorFlag = true
		nontextualerrror = common.NonTextualDataError{Code: "NotNullValidator", Description: "'Non Textual Data' must not be empty."}
	}

	var invalid common.Invalids
	if primaryInfoError.Code != "" {
		fmt.Println(primaryInfoError.Code)
		invalid.PrimaryInfoError = primaryInfoError
		fmt.Println(primaryInfoError, "................................")
	}

	if contentTypeError.Code != "" {
		invalid.ContentTypeError = contentTypeError
	}
	if contentGenresError.Code != "" {
		invalid.ContentGenresError = contentGenresError
		fmt.Println(contentGenresError)
	}
	if contentVarianceError.Code != "" {
		invalid.ContentVarianceError = contentVarianceError
	}
	if casterror.Code != "" {
		invalid.CastError = casterror
	}
	if musicError.Code != "" {
		invalid.MusicError = musicError
	}
	if taginfoError.Code != "" {
		invalid.TaginfoError = taginfoError
	}
	if abouterror.Code != "" {
		invalid.AbouttheContentError = abouterror
	}
	if nontextualerrror.Code != "" {
		invalid.NonTextualDataError = nontextualerrror
	}
	// if arabicTitleError.Code != "" {
	// 	invalid.ArabicTitleError = arabicTitleError
	// }
	// if transliratederr.Code != "" {
	// 	invalid.EnglishTitleError = transliratederr
	// }
	var finalErrorResponse common.FinalErrorResponse
	finalErrorResponse = common.FinalErrorResponse{Error: "invalid_request", Description: "Validation failed.", Code: "error_validation_failed", RequestId: randstr.String(32), Invalid: invalid}
	if errorFlag {
		c.JSON(http.StatusBadRequest, finalErrorResponse)
		return
	}

	errorresponse := common.ServerErrorResponse()
	//	var contentVariance ContentVariance
	var contentRights ContentRights
	var contentTranslation ContentTranslation
	var contentRightsCountry ContentRightsCountry
	var varianceTrailer VarianceTrailer
	var aboutTheContentInfo AboutTheContentInfo
	var contentCast ContentCast
	var playbackItem PlaybackItem
	//var contentGenre ContentGenre
	var playbackItemTargetPlatform PlaybackItemTargetPlatform
	//	var content Content
	var contentRightsPlan ContentRightsPlan
	var rightsProduct RightsProduct
	var productionCountry ProductionCountry
	var seoDetailsResponse Content
	ctx := context.Background()
	tx := db.Debug().BeginTx(ctx, nil)
	var statusdetails StatusDetails
	var newarray []int
	for _, data := range *request.TextualData.ContentVariances {
		var ditalarray []int
		for _, data := range data.DigitalRightsRegions {

			ditalarray = append(ditalarray, data)
		}

		newarray = append(newarray, ditalarray...)
	}

	var errorFlags bool
	errorFlags = RemoveDuplicateValues(newarray)
	if errorFlags {
		c.JSON(http.StatusBadRequest, common.ServerError{Error: "countries exists", Description: "Selected countries for this variant are not allowed.", Code: "", RequestId: randstr.String(32)})
		return
	}
	if c.Param("id") != "" {
		tx.Debug().Table("content").Select("id,status,cast_id,music_id,tag_info_id").Where("id=?", c.Param("id")).Find(&statusdetails)
	}
	fmt.Println(statusdetails, "............")
	primaryinforesponse := request.TextualData.PrimaryInfo
	primaryupdate := ContentPrimaryInfo{OriginalTitle: primaryinforesponse.OriginalTitle, AlternativeTitle: primaryinforesponse.AlternativeTitle, ArabicTitle: primaryinforesponse.ArabicTitle, TransliteratedTitle: primaryinforesponse.TransliteratedTitle, Notes: primaryinforesponse.Notes, IntroStart: primaryinforesponse.IntroStart, OutroStart: primaryinforesponse.OutroStart}

	actorsdata := request.TextualData.Cast
	contentCast = ContentCast{MainActorId: actorsdata.MainActorId, MainActressId: actorsdata.MainActressId}
	fmt.Println(contentCast, "content cast")
	if c.Param("id") == "" {
		if res := tx.Debug().Table("content_cast").Create(&contentCast).Error; res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
			return
		}
	} else {
		if res := tx.Debug().Table("content_cast").Where("id=?", statusdetails.CastId).Update(contentCast).Error; res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
			return
		}
	}
	var contentmusic ContentMusic
	if c.Param("id") == "" {
		if res := tx.Debug().Table("content_music").Create(&contentmusic).Error; res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
			return
		}
	}
	// actor
	var contentactorfinal []interface{}
	if len(request.TextualData.Cast.Actors) > 0 {
		for _, actorrange := range request.TextualData.Cast.Actors {

			contentactor := ContentActor{CastId: contentCast.Id, ActorId: actorrange}
			contentactorfinal = append(contentactorfinal, contentactor)
		}
	}
	var actorfinal []interface{}
	if statusdetails.Id == "" {
		if res := gormbulk.BulkInsert(tx, contentactorfinal, common.BULK_INSERT_LIMIT); res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
		}
	} else {
		var contentactordetails ContentActor
		if res := tx.Debug().Table("content_actor").Where("cast_id=?", statusdetails.CastId).Delete(&contentactordetails).Error; res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
		}
		for _, actorrange := range request.TextualData.Cast.Actors {
			contentactorfinal := ContentActor{CastId: statusdetails.CastId, ActorId: actorrange}
			actorfinal = append(actorfinal, contentactorfinal)
		}
	}
	if statusdetails.Id != "" && len(request.TextualData.Cast.Actors) > 0 {
		if res := gormbulk.BulkInsert(tx, actorfinal, common.BULK_INSERT_LIMIT); res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
		}
	}
	// writer
	var contentwriterfinal []interface{}
	if len(request.TextualData.Cast.Writers) > 0 {
		for _, actorrange := range request.TextualData.Cast.Writers {

			contentwriter := ContentWriter{CastId: contentCast.Id, WriterId: actorrange}
			contentwriterfinal = append(contentwriterfinal, contentwriter)
		}
	}
	var writerfinal []interface{}
	if statusdetails.Id == "" {
		if res := gormbulk.BulkInsert(tx, contentwriterfinal, common.BULK_INSERT_LIMIT); res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
		}
	} else {
		var contentactordetails ContentActor
		if res := tx.Debug().Table("content_writer").Where("cast_id=?", statusdetails.CastId).Delete(&contentactordetails).Error; res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
		}
		for _, writerrange := range request.TextualData.Cast.Writers {
			contentwriter := ContentWriter{CastId: statusdetails.CastId, WriterId: writerrange}
			writerfinal = append(writerfinal, contentwriter)
		}
	}
	if statusdetails.Id != "" && len(request.TextualData.Cast.Writers) > 0 {
		if res := gormbulk.BulkInsert(tx, writerfinal, common.BULK_INSERT_LIMIT); res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
		}
	}
	// director
	var contentdirectorfinal []interface{}
	if len(request.TextualData.Cast.Directors) > 0 {
		for _, actorrange := range request.TextualData.Cast.Directors {

			contentwriter := ContentDirector{CastId: contentCast.Id, DirectorId: actorrange}
			contentdirectorfinal = append(contentdirectorfinal, contentwriter)
		}
	}
	var directorfinal []interface{}
	if statusdetails.Id == "" {
		if res := gormbulk.BulkInsert(tx, contentdirectorfinal, common.BULK_INSERT_LIMIT); res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
		}
	} else {
		var contentdirectordetails ContentDirector
		if res := tx.Debug().Table("content_director").Where("cast_id=?", statusdetails.CastId).Delete(&contentdirectordetails).Error; res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
		}
		for _, directorrange := range request.TextualData.Cast.Directors {
			contentdirector := ContentDirector{CastId: statusdetails.CastId, DirectorId: directorrange}
			directorfinal = append(directorfinal, contentdirector)
		}
	}
	if statusdetails.Id != "" && len(request.TextualData.Cast.Directors) > 0 {
		if res := gormbulk.BulkInsert(tx, directorfinal, common.BULK_INSERT_LIMIT); res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
		}
	}
	// singer
	var contentsingerrfinal []interface{}
	if len(request.TextualData.Music.Singers) > 0 {
		for _, musicrange := range request.TextualData.Music.Singers {

			contentsinger := ContentSinger{MusicId: contentmusic.Id, SingerId: musicrange}
			contentsingerrfinal = append(contentsingerrfinal, contentsinger)
		}
	}
	var musicfinal []interface{}
	if statusdetails.Id == "" {
		if res := gormbulk.BulkInsert(tx, contentsingerrfinal, common.BULK_INSERT_LIMIT); res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
		}
	} else {
		var contentsingerdetails ContentSinger
		if res := tx.Debug().Table("content_singer").Where("music_id=?", statusdetails.MusicId).Delete(&contentsingerdetails).Error; res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
		}
		for _, musicrange := range request.TextualData.Music.Singers {
			contentsinger := ContentSinger{MusicId: statusdetails.MusicId, SingerId: musicrange}
			musicfinal = append(musicfinal, contentsinger)
		}
	}
	if statusdetails.Id != "" && len(request.TextualData.Music.Singers) > 0 {
		if res := gormbulk.BulkInsert(tx, musicfinal, common.BULK_INSERT_LIMIT); res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
		}
	}
	// music_composer
	var contentmusiccomposerrfinal []interface{}
	if len(request.TextualData.Music.MusicComposers) > 0 {
		for _, musicrange := range request.TextualData.Music.MusicComposers {

			contentmusiccomposer := ContentMusicComposer{MusicId: contentmusic.Id, MusicComposerId: musicrange}
			contentmusiccomposerrfinal = append(contentmusiccomposerrfinal, contentmusiccomposer)
		}
	}
	var musiccomposerfinal []interface{}
	if statusdetails.Id == "" {
		if res := gormbulk.BulkInsert(tx, contentmusiccomposerrfinal, common.BULK_INSERT_LIMIT); res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
		}
	} else {
		var contentmusiccomposerdetails ContentMusicComposer
		if res := tx.Debug().Table("content_music_composer").Where("music_id=?", statusdetails.MusicId).Delete(&contentmusiccomposerdetails).Error; res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
		}
		for _, musicrange := range request.TextualData.Music.MusicComposers {
			contentmusiccomposer := ContentMusicComposer{MusicId: statusdetails.MusicId, MusicComposerId: musicrange}
			musiccomposerfinal = append(musiccomposerfinal, contentmusiccomposer)
		}
	}
	if statusdetails.Id != "" && len(request.TextualData.Music.MusicComposers) > 0 {
		if res := gormbulk.BulkInsert(tx, musiccomposerfinal, common.BULK_INSERT_LIMIT); res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
		}
	}
	// song writer
	var contentsongwriterfinal []interface{}
	if len(request.TextualData.Music.SongWriters) > 0 {
		for _, songrange := range request.TextualData.Music.SongWriters {
			contentsongwriter := ContentSongWriter{MusicId: contentmusic.Id, SongWriterId: songrange}
			contentsongwriterfinal = append(contentsongwriterfinal, contentsongwriter)
		}
	}
	var songfinal []interface{}
	if statusdetails.Id == "" {
		if res := gormbulk.BulkInsert(tx, contentsongwriterfinal, common.BULK_INSERT_LIMIT); res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
		}
	} else {
		var contentsongwriterdetails ContentSongWriter
		if res := tx.Debug().Table("content_song_writer").Where("music_id=?", statusdetails.MusicId).Delete(&contentsongwriterdetails).Error; res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
		}
		for _, songrange := range request.TextualData.Music.SongWriters {
			contentsongwriter := ContentSongWriter{MusicId: statusdetails.MusicId, SongWriterId: songrange}
			songfinal = append(songfinal, contentsongwriter)
		}
	}
	if statusdetails.Id != "" && len(request.TextualData.Music.SongWriters) > 0 {
		if res := gormbulk.BulkInsert(tx, songfinal, common.BULK_INSERT_LIMIT); res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
		}
	}
	seoresponse := request.TextualData.SeoDetails
	updateresponse := SeoDetailsResponse{ContentType: primaryinforesponse.ContentType, EnglishMetaTitle: seoresponse.EnglishMetaTitle, ArabicMetaTitle: seoresponse.ArabicMetaTitle, EnglishMetaDescription: seoresponse.EnglishMetaDescription, ArabicMetaDescription: seoresponse.ArabicMetaDescription, Status: 3, ModifiedAt: time.Now()}
	// var primaryInfoIdDetails PrimaryInfoIdDetails
	var Variances []Variance
	var variance Variance
	if c.Param("id") == "" {
		var contentKeyResponse ContentKeyResponse
		if contentkeyresult := tx.Debug().Table("content").Select("max(content_key) as content_key").Find(&contentKeyResponse).Error; contentkeyresult != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"message": contentkeyresult.Error(), "status": http.StatusInternalServerError})
		}
		// To Do temporarily disabled
		contentkey := request.ContentKey
		// contentkey := contentKeyResponse.ContentKey + 1
		seoDetailsResponse.ContentKey = contentkey
		fmt.Println(contentkey, "content key is")
		seoDetailsResponse.ContentType = primaryinforesponse.ContentType
		// for creating old contents with .net take content id and createdby userid from request body
		seoDetailsResponse.Id = request.ContentId
		//	seoDetailsResponse.CreatedByUserId = request.CreatedByUserId

		seoDetailsResponse.Status = 3

		if request.NonTextualData.PosterImage != "" {
			seoDetailsResponse.HasPosterImage = true
		} else {
			seoDetailsResponse.HasPosterImage = false
		}
		if request.NonTextualData.DetailsBackground != "" {
			seoDetailsResponse.HasDetailsBackground = true
		} else {
			seoDetailsResponse.HasDetailsBackground = false
		}
		if request.NonTextualData.MobileDetailsBackground != "" {
			seoDetailsResponse.HasMobileDetailsBackground = true
		} else {
			seoDetailsResponse.HasMobileDetailsBackground = false
		}
		seoDetailsResponse.ContentTier = 1
		seoDetailsResponse.CreatedAt = time.Now()
		seoDetailsResponse.ModifiedAt = time.Now()
		seoDetailsResponse.EnglishMetaTitle = seoresponse.EnglishMetaTitle
		seoDetailsResponse.ArabicMetaTitle = seoresponse.ArabicMetaTitle
		seoDetailsResponse.EnglishMetaDescription = seoresponse.EnglishMetaDescription
		seoDetailsResponse.ArabicMetaDescription = seoresponse.ArabicMetaDescription
		seoDetailsResponse.PrimaryInfoId = "00000000-0000-0000-0000-000000000000"
		seoDetailsResponse.AboutTheContentInfoId = "00000000-0000-0000-0000-000000000000"
		seoDetailsResponse.CastId = contentCast.Id
		seoDetailsResponse.MusicId = contentmusic.Id
		seoDetailsResponse.TagInfoId = "00000000-0000-0000-0000-000000000000"
		// seoDetailsResponse.DeletedByUserId = nil
		seoDetailsResponse.CreatedByUserId = userid.(string)

		if primaryinfoupdate := tx.Debug().Table("content_primary_info").Create(&primaryupdate).Error; primaryinfoupdate != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"message": primaryinfoupdate.Error(), "status": http.StatusInternalServerError})
			return
		}
		// if contentpriamryinfoupdate := tx.Table("content").Where("id=?", seoDetailsResponse.Id).Update("primary_info_id", primaryupdate.Id).Error; contentpriamryinfoupdate != nil {
		// 	tx.Rollback()
		// 	c.JSON(http.StatusInternalServerError, gin.H{"message": contentpriamryinfoupdate.Error(), "status": http.StatusInternalServerError})
		// 	return
		// }
		if contentupdate := tx.Debug().Table("content").Create(&seoDetailsResponse).Error; contentupdate != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
			return
		}

		for i, data := range *request.TextualData.ContentGenres {

			contentresponse := ContentGenreResponse{ContentId: seoDetailsResponse.Id, Order: i + 1, GenreId: data.GenreId}
			if genreupdate := tx.Debug().Table("content_genre").Create(&contentresponse).Error; genreupdate != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"message": genreupdate.Error(), "status": http.StatusInternalServerError})
				return
			}

			for i, value := range data.SubgenresId {
				subgenreresponse := SubGenreResponse{ContentGenreId: contentresponse.Id, Order: i + 1, SubgenreId: value}
				if subgenreupdate := tx.Debug().Table("content_subgenre").Create(&subgenreresponse).Error; subgenreupdate != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, gin.H{"message": subgenreupdate.Error(), "status": http.StatusInternalServerError})
				}
			}
		}
		for i, data := range *request.TextualData.ContentVariances {
			contentTranslation = ContentTranslation{LanguageType: common.ContentLanguageOriginTypes(data.LanguageType), DubbingLanguage: data.DubbingLanguage, DubbingDialectId: data.DubbingDialectId, SubtitlingLanguage: data.SubtitlingLanguage}
			if res := tx.Debug().Table("content_translation").Create(&contentTranslation).Error; res != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, errorresponse)
				return
			}
			contentRights = ContentRights{DigitalRightsType: data.DigitalRightsType, DigitalRightsStartDate: data.DigitalRightsStartDate, DigitalRightsEndDate: data.DigitalRightsEndDate}
			if contentrightsres := tx.Debug().Table("content_rights").Create(&contentRights).Error; contentrightsres != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, errorresponse)
				return
			}
			var newarr []interface{}
			for _, value := range data.DigitalRightsRegions {

				contentRightsCountry = ContentRightsCountry{ContentRightsId: contentRights.Id, CountryId: value}
				fmt.Println(contentRightsCountry, "content country is")
				newarr = append(newarr, contentRightsCountry)
			}
			if res := gormbulk.BulkInsert(tx, newarr, common.BULK_INSERT_LIMIT); res != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, errorresponse)
			}
			_, _, duration := common.GetVideoDuration(data.VideoContentId)
			if duration == 0 {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "InValid Content VideoId", "description": "Please provide valid Video ContentId", "code": "", "requestId": randstr.String(32)})
				return
			}
			// take created by userid from request body for creating old contents else take user id from generated token
			playbackItem = PlaybackItem{VideoContentId: data.VideoContentId, TranslationId: contentTranslation.Id, RightsId: contentRights.Id, CreatedByUserId: userid.(string), SchedulingDateTime: data.SchedulingDateTime}
			if res := tx.Debug().Table("playback_item").Create(&playbackItem).Error; res != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, errorresponse)
				return
			}

			var contentVariance ContentVariance

			// for sync add content varince id
			if len(request.VarianceIds) > 0 {
				contentVariance.ID = request.VarianceIds[i]
			}
			if data.OverlayPosterImage != "" {
				contentVariance.HasOverlayPosterImage = true
			} else {
				contentVariance.HasOverlayPosterImage = false
			}
			if data.DubbingScript != "" {
				contentVariance.HasDubbingScript = true
			} else {
				contentVariance.HasDubbingScript = false
			}
			if data.SubtitlingScript != "" {
				contentVariance.HasSubtitlingScript = true
			} else {
				contentVariance.HasSubtitlingScript = false
			}
			contentVariance.IntroDuration = data.IntroDuration
			contentVariance.IntroStart = data.IntroStart
			contentVariance.ContentId = seoDetailsResponse.Id
			contentVariance.Status = 3
			contentVariance.CreatedAt = time.Now()
			contentVariance.ModifiedAt = time.Now()
			//	contentVariance.DeletedByUserId = "00000000-0000-0000-0000-000000000000"
			contentVariance.ContentId = seoDetailsResponse.Id
			if playbackItem.Id != "" {
				contentVariance.PlaybackItemId = playbackItem.Id
			} else {
				contentVariance.PlaybackItemId = "00000000-0000-0000-0000-000000000000"
			}
			contentVariance.Order = i + 1
			fmt.Println(contentVariance.ContentId)
			fmt.Println(contentVariance.PlaybackItemId)

			if res := tx.Debug().Table("content_variance").Create(&contentVariance).Error; res != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, errorresponse)
				return
			}
			if len(data.VarianceTrailers) != 0 {
				for i, a := range data.VarianceTrailers {
					if a.VideoTrailerId != "" {
						_, _, duration := common.GetVideoDuration(a.VideoTrailerId)
						if duration == 0 {
							c.JSON(http.StatusInternalServerError, gin.H{"error": "InValid Content TrailerId", "description": "Please provide valid Video TrailerId", "code": "", "requestId": randstr.String(32)})
							return
						}
						//	varianceTrailer = VarianceTrailer{Order: i + 1, VideoTrailerId: a.VideoTrailerId, EnglishTitle: a.EnglishTitle, ArabicTitle: a.ArabicTitle, Duration: duration, HasTrailerPosterImage: a.HasTrailerPosterImage, ContentVarianceId: contentVariance.ID}
						var varianceTrailer VarianceTrailer
						// for sync
						if len(data.VarianceTrailerIds) > 0 {
							varianceTrailer.Id = data.VarianceTrailerIds[i]
						}
						varianceTrailer.Order = i + 1
						varianceTrailer.VideoTrailerId = a.VideoTrailerId
						varianceTrailer.EnglishTitle = a.EnglishTitle
						varianceTrailer.ArabicTitle = a.ArabicTitle
						varianceTrailer.Duration = duration
						if a.TrailerPosterImage != "" {
							varianceTrailer.HasTrailerPosterImage = true
						} else {
							varianceTrailer.HasTrailerPosterImage = false
						}
						varianceTrailer.ContentVarianceId = contentVariance.ID
						if res := tx.Debug().Table("variance_trailer").Create(&varianceTrailer).Error; res != nil {
							tx.Rollback()
							c.JSON(http.StatusInternalServerError, errorresponse)
							return
						}
						go ContentVarianceTrailerImageUpload(seoDetailsResponse.Id, contentVariance.ID, varianceTrailer.Id, a.TrailerPosterImage)
					}
				}
			}
			if playbackItem.Id != "" {
				var publishplatform []interface{}
				for _, publishrange := range data.PublishingPlatforms {
					playbackItemTargetPlatform = PlaybackItemTargetPlatform{PlaybackItemId: playbackItem.Id, TargetPlatform: publishrange, RightsId: contentRights.Id}
					publishplatform = append(publishplatform, playbackItemTargetPlatform)
				}
				if res := gormbulk.BulkInsert(tx, publishplatform, common.BULK_INSERT_LIMIT); res != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, errorresponse)
					return
				}
			}
			if len(data.SubscriptionPlans) > 0 {
				for _, contentplanrange := range data.SubscriptionPlans {
					contentRightsPlan = ContentRightsPlan{RightsId: contentRights.Id, SubscriptionPlanId: contentplanrange}
					if res := tx.Debug().Table("content_rights_plan").Create(&contentRightsPlan).Error; res != nil {
						tx.Rollback()
						c.JSON(http.StatusInternalServerError, errorresponse)
						return
					}
				}
			}
			if len(data.Products) > 0 {
				for _, productrange := range data.Products {
					rightsProduct = RightsProduct{RightsId: contentRights.Id, ProductName: productrange}
					if res := tx.Debug().Table("rights_product").Create(&rightsProduct).Error; res != nil {
						tx.Rollback()
						c.JSON(http.StatusInternalServerError, errorresponse)
						return
					}
				}
			}
			variance.Id = contentVariance.ID
			variance.OverlayPosterImage = data.OverlayPosterImage
			variance.DubbingScript = data.DubbingScript
			variance.SubtitlingScript = data.SubtitlingScript
			Variances = append(Variances, variance)
		}
		about := request.TextualData.AboutTheContent
		aboutTheContentInfo = AboutTheContentInfo{OriginalLanguage: about.OriginalLanguage, Supplier: about.Supplier, AcquisitionDepartment: about.AcquisitionDepartment, EnglishSynopsis: about.EnglishSynopsis, ArabicSynopsis: about.ArabicSynopsis, ProductionYear: about.ProductionYear, ProductionHouse: about.ProductionHouse, AgeGroup: about.AgeGroup}
		fmt.Println(aboutTheContentInfo, "about the conent info is")
		if res := tx.Debug().Table("about_the_content_info").Create(&aboutTheContentInfo).Error; res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
			return
		}
		fmt.Println(aboutTheContentInfo.Id)
		if len(about.ProductionCountries) > 0 {
			for _, productionrange := range about.ProductionCountries {
				productionCountry = ProductionCountry{AboutTheContentInfoId: aboutTheContentInfo.Id, CountryId: productionrange}
				if res := tx.Debug().Table("production_country").Create(&productionCountry).Error; res != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, errorresponse)
					return
				}
			}
		}
		var contentTagInfo ContentTagInfo
		if res := tx.Debug().Table("content_tag_info").Create(&contentTagInfo).Error; res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
			return
		}
		var contentTag ContentTag
		for _, tagrange := range request.TextualData.TagInfo.Tags {
			contentTag.TagInfoId = contentTagInfo.Id
			contentTag.TextualDataTagId = tagrange
			if res := tx.Debug().Table("content_tag").Create(&contentTag).Error; res != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, errorresponse)
				return
			}
		}
		var contentupdate Content
		contentupdate.AboutTheContentInfoId = aboutTheContentInfo.Id
		contentupdate.PrimaryInfoId = primaryupdate.Id
		contentupdate.TagInfoId = contentTagInfo.Id
		if res := tx.Debug().Table("content").Where("id=?", seoDetailsResponse.Id).Update(contentupdate).Error; res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
			return
		}
		// contentVariances := ContentVariance{ContentId: seoDetailsResponse.Id, PlaybackItemId: playbackItem.Id}
		// if res := tx.Table("content_variance").Where("id=?", contentVariance.ID).Update(contentVariances).Error; res != nil {
		// 	tx.Rollback()
		// 	c.JSON(http.StatusInternalServerError, errorresponse)
		// 	return
		// }
		id := map[string]string{"id": seoDetailsResponse.Id}
		/* upload images to S3 bucket based on content onetier Id*/
		ContentFileUpload(request, seoDetailsResponse.Id)
		ContentVarianceImageUpload(Variances, seoDetailsResponse.Id)
		c.JSON(http.StatusOK, gin.H{"data": id})
		// update one tier
	} else {
		fmt.Println("-------------------update one tier------------------")
		var primaryInfoIdDetails Content
		if primaryinfoid := tx.Debug().Table("content").Select("primary_info_id,about_the_content_info_id").Where("id=?", c.Param("id")).Find(&primaryInfoIdDetails).Error; primaryinfoid != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
			return
		}
		if primaryinfoupdate := tx.Debug().Table("content_primary_info").Where("id=?", primaryInfoIdDetails.PrimaryInfoId).Update(primaryupdate).Error; primaryinfoupdate != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
			return
		}

		if contentupdate := tx.Debug().Table("content").Where("id=?", c.Param("id")).Update(updateresponse).Error; contentupdate != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
			return
		}
		var contentgenreid ContentGenre
		if res := tx.Debug().Table("content_genre").Select("id").Where("content_id=?", c.Param("id")).Find(&contentgenreid).Error; res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
			return
		}
		var contentgenre ContentGenre
		tx.Debug().Table("content_genre").Where("content_id=?", c.Param("id")).Delete(&contentgenre)

		var contentsubgenre ContentSubgenre
		tx.Debug().Table("content_subgenre").Where("content_genre_id=?", contentgenreid.Id).Delete(&contentsubgenre)

		// var order int
		// order = 0
		for i, data := range *request.TextualData.ContentGenres {
			//	order = order + 1
			contentgenrecreate := ContentGenre{ContentId: c.Param("id"), Order: i + 1, GenreId: data.GenreId}
			if genreupdate := tx.Debug().Table("content_genre").Create(&contentgenrecreate).Error; genreupdate != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, errorresponse)
				return
			}
			// var contentgenrid []ContentgenreId
			// if res := tx.Raw("select id from content_genre where content_id=?", c.Param("id")).Find(&contentgenrid).Error; res != nil {
			// 	tx.Rollback()
			// 	c.JSON(http.StatusInternalServerError, errorresponse)
			// }

			for i, value := range data.SubgenresId {
				subgenreresponse := SubGenreResponse{ContentGenreId: contentgenrecreate.Id, Order: i + 1, SubgenreId: value}
				if subgenreupdate := tx.Debug().Table("content_subgenre").Create(&subgenreresponse).Error; subgenreupdate != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, gin.H{"message": subgenreupdate.Error(), "status": http.StatusInternalServerError})
				}
			}
		}
		// deleting not existing variances
		var existingvariances []ContentVariance
		if err := tx.Debug().Table("content_variance").Select("id").Where("content_id=? and deleted_by_user_id is null ", c.Param("id")).Find(&existingvariances).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
			return
		}
		var newarrays []string
		var exist bool
		fmt.Println(newarrays, ";;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;")

		for _, varaincerange := range existingvariances {
			exist = false
			for _, newvariances := range *request.TextualData.ContentVariances {
				if varaincerange.ID == newvariances.Id {
					fmt.Println(varaincerange.ID, ";;;;;;;")
					fmt.Println(newvariances.Id, ";;;;;;;;;;;;;")
					fmt.Println(exist, ";;;;;;;;;;;;;;;;;;")
					exist = true
					break
				}
			}
			if !exist {
				newarrays = append(newarrays, varaincerange.ID)
			}
		}
		fmt.Println(newarrays, "kkkkkkkkkkkkkkkkkk----------------------------")
		for _, variances := range newarrays {

			tx.Debug().Table("content_variance").Where("content_id=? and id=?", c.Param("id"), variances).Update("deleted_by_user_id", userid.(string))
		}

		var varainceoreders int
		varainceoreders = 0
		for i, value := range *request.TextualData.ContentVariances {

			// var count int
			// tx.Table("content_variance").Select("id").Where("content_id=?", c.Param("id")).Count(&count)
			// fmt.Println(count, "count is")
			if value.Id != "" {
				var contentVariance ContentVariance
				contentVariance.Status = 3
				if value.OverlayPosterImage != "" {
					contentVariance.HasOverlayPosterImage = true
				} else {
					contentVariance.HasOverlayPosterImage = false
				}
				if value.DubbingScript != "" {
					contentVariance.HasDubbingScript = true
				} else {
					contentVariance.HasDubbingScript = false
				}
				if value.SubtitlingScript != "" {
					contentVariance.HasSubtitlingScript = true
				} else {
					contentVariance.HasSubtitlingScript = false
				}
				contentVariance.IntroDuration = value.IntroDuration
				contentVariance.IntroStart = value.IntroStart
				contentVariance.ContentId = seoDetailsResponse.Id
				contentVariance.ModifiedAt = time.Now()
				if res := tx.Debug().Table("content_variance").Where("content_id=? and id=?", c.Param("id"), value.Id).Update(contentVariance).Error; res != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, errorresponse)
					return
				}
				var contentvdetails ContentVariance
				if res := tx.Debug().Table("content_variance").Select("playback_item_id,id").Where("content_id=? and id=?", c.Param("id"), value.Id).Find(&contentvdetails).Error; res != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, errorresponse)
					return
				}
				_, _, duration := common.GetVideoDuration(value.VideoContentId)
				if duration == 0 {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "InValid Content VideoId", "description": "Please provide valid Video ContentId", "code": "", "requestId": randstr.String(32)})
					return
				}
				playbackItemreq := PlaybackItem{VideoContentId: value.VideoContentId, SchedulingDateTime: value.SchedulingDateTime, Duration: duration}
				// new implementation
				if value.SchedulingDateTime == nil {
					if res := tx.Debug().Table("playback_item").Select("scheduling_date_time").Where("id=?", contentvdetails.PlaybackItemId).Updates(map[string]interface{}{"scheduling_date_time": gorm.Expr("NULL")}).Error; res != nil {
						tx.Rollback()
						c.JSON(http.StatusInternalServerError, errorresponse)
						return
					}
					if res := tx.Debug().Table("playback_item").Where("id=?", contentvdetails.PlaybackItemId).Update(playbackItemreq).Error; res != nil {
						tx.Rollback()
						c.JSON(http.StatusInternalServerError, errorresponse)
						return
					}

				} else {
					if res := tx.Debug().Table("playback_item").Where("id=?", contentvdetails.PlaybackItemId).Update(playbackItemreq).Error; res != nil {
						tx.Rollback()
						c.JSON(http.StatusInternalServerError, errorresponse)
						return
					}
				}
				if res := tx.Debug().Table("playback_item").Select("rights_id,translation_id").Where("id=?", contentvdetails.PlaybackItemId).Find(&playbackItem).Error; res != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, errorresponse)
					return
				}
				contentrights := ContentRights{DigitalRightsType: value.DigitalRightsType, DigitalRightsEndDate: value.DigitalRightsEndDate, DigitalRightsStartDate: value.DigitalRightsStartDate}
				if res := tx.Debug().Table("content_rights").Where("id=?", playbackItem.RightsId).Update(contentrights).Error; res != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, errorresponse)
					return
				}
				if res := tx.Debug().Table("content_rights_country").Where("content_rights_id=?", playbackItem.RightsId).Delete(&contentRightsCountry).Error; res != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, errorresponse)
					return
				}
				var regions []interface{}
				for _, data := range value.DigitalRightsRegions {

					contentrightcountry := ContentRightsCountry{ContentRightsId: playbackItem.RightsId, CountryId: data}
					regions = append(regions, contentrightcountry)
				}
				if res := gormbulk.BulkInsert(tx, regions, common.BULK_INSERT_LIMIT); res != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, errorresponse)

				}
				if res := tx.Debug().Table("playback_item_target_platform").Where("playback_item_id=?", contentvdetails.PlaybackItemId).Delete(&playbackItem).Error; res != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, errorresponse)
					return
				}
				var publishplatformarr []interface{}
				for _, publishplatformrange := range value.PublishingPlatforms {
					playbackItemTargetPlatforms := PlaybackItemTargetPlatform{PlaybackItemId: contentvdetails.PlaybackItemId, TargetPlatform: publishplatformrange, RightsId: playbackItem.RightsId}
					publishplatformarr = append(publishplatformarr, playbackItemTargetPlatforms)
				}
				if res := gormbulk.BulkInsert(tx, publishplatformarr, common.BULK_INSERT_LIMIT); res != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, errorresponse)

				}
				var contenttranslation ContentTranslation
				contenttranslation.LanguageType = common.ContentLanguageOriginTypes(value.LanguageType)
				contenttranslation.DubbingDialectId = value.DubbingDialectId
				contenttranslation.DubbingLanguage = value.DubbingLanguage
				contenttranslation.SubtitlingLanguage = value.SubtitlingLanguage

				if res := tx.Debug().Table("content_translation").Where("id=?", playbackItem.TranslationId).Update(contenttranslation).Error; res != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, errorresponse)
					return
				}

				tx.Debug().Table("content_rights_plan").Where("rights_id=?", playbackItem.RightsId).Delete(&ContentRightsPlan{})

				if len(value.SubscriptionPlans) > 0 {
					for _, contentplanrange := range value.SubscriptionPlans {
						contentrightsplans := ContentRightsPlan{RightsId: playbackItem.RightsId, SubscriptionPlanId: contentplanrange}
						if res := tx.Debug().Table("content_rights_plan").Create(&contentrightsplans).Error; res != nil {
							tx.Rollback()
							c.JSON(http.StatusInternalServerError, errorresponse)
							return
						}
					}
				}

				var rights RightsProduct

				tx.Debug().Table("rights_product").Where("rights_id=?", playbackItem.RightsId).Delete(&rights)

				if len(value.Products) > 0 {
					for _, productrange := range value.Products {
						fmt.Println(productrange, "kkkkkkkkkkkk")
						rightsprodu := RightsProduct{RightsId: playbackItem.RightsId, ProductName: productrange}
						if err := tx.Debug().Table("rights_product").Create(&rightsprodu).Error; err != nil {
							c.JSON(http.StatusInternalServerError, errorresponse)
							return
						}
					}
				}
				// deleting not extisting trailer
				var existingtrailers []VarianceTrailer
				var newtrailers []string
				var exists bool
				if len(value.VarianceTrailers) > 0 {
					tx.Debug().Table("variance_trailer").Select("id").Where("content_variance_id=?", value.Id).Find(&existingtrailers)
					for _, varaincetrailers := range existingtrailers {
						exists = false
						for _, existing := range value.VarianceTrailers {
							if varaincetrailers.Id == existing.Id {
								exists = true
								break
							}
						}
						if !exists {
							newtrailers = append(newtrailers, varaincetrailers.Id)
						}
					}
					if len(newtrailers) >= 1 {
						for _, new := range newtrailers {
							tx.Debug().Table("variance_trailer").Where("content_variance_id=? and id=?", value.Id, new).Delete(&varianceTrailer)
						}
					}
				} else {
					tx.Debug().Table("variance_trailer").Where("content_variance_id=?", value.Id).Delete(&varianceTrailer)
				}
				var trailerorders int
				trailerorders = 0
				for i, variancerange := range value.VarianceTrailers {
					// var totalcount int
					// fmt.Println(value.Id, "kkkkkkkkkkkkkkkkkkk")
					// tx.Debug().Table("variance_trailer").Select("id").Where("id=?", variancerange.Id).Count(&totalcount)
					// fmt.Println(totalcount, "total count is")
					if variancerange.Id != "" {

						// variance trailer update
						_, _, duration := common.GetVideoDuration(variancerange.VideoTrailerId)
						if duration == 0 {
							c.JSON(http.StatusInternalServerError, gin.H{"error": "InValid Content TrailerId", "description": "Please provide valid Video TrailerId", "code": "", "requestId": randstr.String(32)})
							return
						}

						varinceupdattes := VarianceTrailer{Order: variancerange.Order, VideoTrailerId: variancerange.VideoTrailerId, EnglishTitle: variancerange.EnglishTitle, ArabicTitle: variancerange.ArabicTitle, Duration: duration, HasTrailerPosterImage: variancerange.HasTrailerPosterImage}
						if res := tx.Debug().Table("variance_trailer").Where("content_variance_id=? and id=?", value.Id, variancerange.Id).Update(varinceupdattes).Error; res != nil {
							c.JSON(http.StatusInternalServerError, errorresponse)
							return
						}
						go ContentVarianceTrailerImageUpload(c.Param("id"), contentvdetails.ID, variancerange.Id, variancerange.TrailerPosterImage)
					} else {
						// create variance trailer
						if variancerange.VideoTrailerId != "" {
							_, _, duration := common.GetVideoDuration(variancerange.VideoTrailerId)
							if duration == 0 {
								c.JSON(http.StatusInternalServerError, gin.H{"error": "InValid Content TrailerId", "description": "Please provide valid Video TrailerId", "code": "", "requestId": randstr.String(32)})
								return
							}
							// varianceTrailer = VarianceTrailer{Order: i + 1, VideoTrailerId: variancerange.VideoTrailerId, EnglishTitle: variancerange.EnglishTitle, ArabicTitle: variancerange.ArabicTitle, Duration: duration, HasTrailerPosterImage: variancerange.HasTrailerPosterImage, ContentVarianceId: value.Id}
							// for sync
							varianceTrailer.Id = value.VarianceTrailerIds[trailerorders]

							varianceTrailer.Order = i + 1
							varianceTrailer.VideoTrailerId = variancerange.VideoTrailerId
							varianceTrailer.EnglishTitle = variancerange.EnglishTitle
							varianceTrailer.ArabicTitle = variancerange.ArabicTitle
							varianceTrailer.Duration = duration
							if variancerange.TrailerPosterImage != "" {
								varianceTrailer.HasTrailerPosterImage = true
							} else {
								varianceTrailer.HasTrailerPosterImage = false
							}
							varianceTrailer.ContentVarianceId = value.Id
							if res := tx.Debug().Table("variance_trailer").Create(&varianceTrailer).Error; res != nil {
								tx.Rollback()
								c.JSON(http.StatusInternalServerError, errorresponse)
								return
							}
							trailerorders = trailerorders + 1
						}
						go ContentVarianceTrailerImageUpload(c.Param("id"), varianceTrailer.ContentVarianceId, varianceTrailer.Id, variancerange.TrailerPosterImage)
					}
				}
				variance.Id = value.Id
				variance.OverlayPosterImage = value.OverlayPosterImage
				variance.DubbingScript = value.DubbingScript
				variance.SubtitlingScript = value.SubtitlingScript
				Variances = append(Variances, variance)
			} else {
				/*Create Variance for onetier-content */
				contentTranslation = ContentTranslation{LanguageType: common.ContentLanguageOriginTypes(value.LanguageType), DubbingLanguage: value.DubbingLanguage, DubbingDialectId: value.DubbingDialectId, SubtitlingLanguage: value.SubtitlingLanguage}
				if res := tx.Debug().Table("content_translation").Create(&contentTranslation).Error; res != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, errorresponse)
					return
				}
				contentRights = ContentRights{DigitalRightsType: value.DigitalRightsType, DigitalRightsStartDate: value.DigitalRightsStartDate, DigitalRightsEndDate: value.DigitalRightsEndDate}
				if contentrightsres := tx.Debug().Table("content_rights").Create(&contentRights).Error; contentrightsres != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, errorresponse)
					return
				}
				var newarr []interface{}
				for _, value := range value.DigitalRightsRegions {

					contentRightsCountry = ContentRightsCountry{ContentRightsId: contentRights.Id, CountryId: value}
					newarr = append(newarr, contentRightsCountry)
				}
				if res := gormbulk.BulkInsert(tx, newarr, common.BULK_INSERT_LIMIT); res != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, errorresponse)
				}

				_, _, duration := common.GetVideoDuration(value.VideoContentId)
				if duration == 0 {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "InValid Content VideoId", "description": "Please provide valid Video ContentId", "code": "", "requestId": randstr.String(32)})
					return
				}
				playbackItem = PlaybackItem{VideoContentId: value.VideoContentId, TranslationId: contentTranslation.Id, Duration: duration, RightsId: contentRights.Id, CreatedByUserId: userid.(string), SchedulingDateTime: value.SchedulingDateTime}
				if res := tx.Debug().Table("playback_item").Create(&playbackItem).Error; res != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, errorresponse)
					return
				}

				var contentVariance ContentVariance
				contentVariance.Status = 3
				// for sync
				contentVariance.ID = request.VarianceIds[varainceoreders]

				if value.OverlayPosterImage != "" {
					contentVariance.HasOverlayPosterImage = true
				} else {
					contentVariance.HasOverlayPosterImage = false
				}
				if value.DubbingScript != "" {
					contentVariance.HasDubbingScript = true
				} else {
					contentVariance.HasDubbingScript = false
				}
				if value.SubtitlingScript != "" {
					contentVariance.HasSubtitlingScript = true
				} else {
					contentVariance.HasSubtitlingScript = false
				}
				contentVariance.IntroDuration = value.IntroDuration
				contentVariance.IntroStart = value.IntroStart
				contentVariance.ContentId = c.Param("id")
				fmt.Println(contentVariance.ContentId, "llllllllllllllllllllll")
				contentVariance.CreatedAt = time.Now()
				contentVariance.ModifiedAt = time.Now()
				//	contentVariance.DeletedByUserId = "00000000-0000-0000-0000-000000000000"
				if playbackItem.Id != "" {
					contentVariance.PlaybackItemId = playbackItem.Id
				} else {
					contentVariance.PlaybackItemId = "00000000-0000-0000-0000-000000000000"
				}
				contentVariance.Order = i + 1
				fmt.Println(contentVariance.ContentId, ".....................")
				fmt.Println(contentVariance.PlaybackItemId)
				//	digitalrights = append(digitalrights, contentVariance)

				if res := tx.Debug().Table("content_variance").Create(&contentVariance).Error; res != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, errorresponse)
					return
				}
				varainceoreders = varainceoreders + 1
				var trailersoreders int
				trailersoreders = 0

				if len(value.VarianceTrailers) != 0 {
					for i, a := range value.VarianceTrailers {
						if a.VideoTrailerId != "" {
							_, _, duration := common.GetVideoDuration(a.VideoTrailerId)
							if duration == 0 {
								c.JSON(http.StatusInternalServerError, gin.H{"error": "InValid Content TrailerId", "description": "Please provide valid Video TrailerId", "code": "", "requestId": randstr.String(32)})
								return
							}
							var varianceTrailer VarianceTrailer
							// for sync
							varianceTrailer.Id = value.VarianceTrailerIds[trailersoreders]

							varianceTrailer.Order = i + 1
							varianceTrailer.VideoTrailerId = a.VideoTrailerId
							varianceTrailer.EnglishTitle = a.EnglishTitle
							varianceTrailer.ArabicTitle = a.ArabicTitle
							varianceTrailer.Duration = duration
							if a.TrailerPosterImage != "" {
								varianceTrailer.HasTrailerPosterImage = true
							} else {
								varianceTrailer.HasTrailerPosterImage = false
							}
							varianceTrailer.ContentVarianceId = contentVariance.ID
							if res := tx.Debug().Table("variance_trailer").Create(&varianceTrailer).Error; res != nil {
								tx.Rollback()
								c.JSON(http.StatusInternalServerError, errorresponse)
								return
							}
							trailersoreders = trailersoreders + 1
							go ContentVarianceTrailerImageUpload(c.Param("id"), varianceTrailer.ContentVarianceId, varianceTrailer.Id, a.TrailerPosterImage)
						}
					}
				}
				if playbackItem.Id != "" {
					var publishplatform []interface{}
					for _, publishrange := range value.PublishingPlatforms {
						playbackItemTargetPlatform = PlaybackItemTargetPlatform{PlaybackItemId: playbackItem.Id, TargetPlatform: publishrange, RightsId: contentRights.Id}
						publishplatform = append(publishplatform, playbackItemTargetPlatform)
					}
					if res := gormbulk.BulkInsert(tx, publishplatform, common.BULK_INSERT_LIMIT); res != nil {
						tx.Rollback()
						c.JSON(http.StatusInternalServerError, errorresponse)
						return
					}
				}
				if len(value.SubscriptionPlans) > 0 {
					for _, contentplanrange := range value.SubscriptionPlans {
						contentRightsPlan = ContentRightsPlan{RightsId: contentRights.Id, SubscriptionPlanId: contentplanrange}
						if res := tx.Debug().Table("content_rights_plan").Create(&contentRightsPlan).Error; res != nil {
							tx.Rollback()
							c.JSON(http.StatusInternalServerError, errorresponse)
							return
						}
					}
				}
				if len(value.Products) > 0 {
					for _, productrange := range value.Products {
						rightsProduct = RightsProduct{RightsId: contentRights.Id, ProductName: productrange}
						if res := tx.Debug().Table("rights_product").Create(&rightsProduct).Error; res != nil {
							tx.Rollback()
							c.JSON(http.StatusInternalServerError, errorresponse)
							return
						}
					}
				}
				variance.Id = contentVariance.ID
				variance.OverlayPosterImage = value.OverlayPosterImage
				variance.DubbingScript = value.DubbingScript
				variance.SubtitlingScript = value.SubtitlingScript
				Variances = append(Variances, variance)

			}
		}
		var aboutthecontentInfo AboutTheContentInfo
		req := request.TextualData.AboutTheContent
		// about := request.TextualData.AboutTheContent
		fmt.Println(req.ProductionYear, "about the content.........", req)
		aboutthecontentInfo.OriginalLanguage = req.OriginalLanguage
		aboutthecontentInfo.Supplier = req.Supplier
		aboutthecontentInfo.AcquisitionDepartment = req.AcquisitionDepartment
		aboutthecontentInfo.EnglishSynopsis = req.EnglishSynopsis
		aboutthecontentInfo.ArabicSynopsis = req.ArabicSynopsis
		aboutthecontentInfo.ProductionYear = req.ProductionYear
		aboutthecontentInfo.ProductionHouse = req.ProductionHouse
		aboutthecontentInfo.AgeGroup = req.AgeGroup
		aboutthecontentInfo.IntroDuration = req.IntroDuration
		aboutthecontentInfo.IntroStart = req.IntroStart
		aboutthecontentInfo.OutroDuration = req.OutroDuration
		aboutthecontentInfo.OutroStart = req.OutroStart

		// var contentabout Content
		// if res := tx.Debug().Table("content").Select("about_the_content_info_id").Where("id=?", c.Param("id")).Find(&contentabout).Error; res != nil {
		// 	c.JSON(http.StatusInternalServerError, errorresponse)
		// 	return
		// }
		fmt.Println("update content info about.......", aboutthecontentInfo, primaryInfoIdDetails.AboutTheContentInfoId)
		if res := tx.Debug().Table("about_the_content_info").Where("id=?", primaryInfoIdDetails.AboutTheContentInfoId).Update(aboutthecontentInfo).Error; res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
			return
		}
		// if res := tx.Table("about_the_content_info ati").Select("c.cast_id,c.about_the_content_info_id").Where("c.id=?", c.Param("id")).Joins("left join content c on ati.id=c.about_the_content_info_id").Find(&content).Error; res != nil {
		// 	c.JSON(http.StatusInternalServerError, errorresponse)
		// 	return
		// }

		if res := tx.Debug().Table("production_country").Where("about_the_content_info_id=?", primaryInfoIdDetails.AboutTheContentInfoId).Delete(&ProductionCountry{}).Error; res != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, errorresponse)
			return
		}

		if len(req.ProductionCountries) > 0 {
			for _, productionrange := range req.ProductionCountries {

				productionCountry = ProductionCountry{AboutTheContentInfoId: primaryInfoIdDetails.AboutTheContentInfoId, CountryId: productionrange}
				if res := tx.Debug().Table("production_country").Create(&productionCountry).Error; res != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, errorresponse)
					return
				}
			}
		}
		var tagfinal []interface{}
		if len(request.TextualData.TagInfo.Tags) > 0 {
			var taginfo TagInfo
			tx.Debug().Table("content_tag").Where("tag_info_id=?", statusdetails.TagInfoId).Delete(&taginfo)
			for _, tagrange := range request.TextualData.TagInfo.Tags {
				contentTagFinal := ContentTag{TagInfoId: statusdetails.TagInfoId, TextualDataTagId: tagrange}
				tagfinal = append(tagfinal, contentTagFinal)
			}
		}
		if len(tagfinal) > 0 {
			if res := gormbulk.BulkInsert(tx, tagfinal, common.BULK_INSERT_LIMIT); res != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, errorresponse)
				return
			}
		}
		id := map[string]string{"id": c.Param("id")}
		/* upload images to S3 bucket based on content onetier Id*/
		ContentFileUpload(request, c.Param("id"))
		ContentVarianceImageUpload(Variances, c.Param("id"))
		/* creating redis keys */
		go common.CreateRedisKeyForContentTypeOTC(c)
		c.JSON(http.StatusOK, gin.H{"data": id})
	}
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, errorresponse)
		return
	}
}

func RemoveDuplicateValues(intSlice []int) bool {
	fmt.Println(intSlice, "...............")
	var errorFlags bool
	errorFlags = false
	// If the key(values of the slice) is not equal
	// to the already present value in new slice (list)
	// then we append it. else we jump on another element.
	intlength := len(intSlice)
	fmt.Println(intlength, "int lenght is")
	for i := 1; i < intlength; i++ {
		for j := i + 1; j < intlength; j++ {
			//	fmt.Println(intSlice[i]==intSlice[j],"jjjjjjjjj")
			if intSlice[i] == intSlice[j] {
				fmt.Println(intSlice[i], intSlice[j], "nnnnnnn")
				errorFlags = true
				fmt.Println(errorFlags, "error flags")
				break
			}
		}
	}
	return errorFlags
}

/*Uploade image in S3 bucket  Based on onetier Id*/
func ContentFileUpload(request OnetierContentRequest, contentId string) {
	bucketName := os.Getenv("S3_BUCKET")
	var newarr []string
	// newarr = append(newarr, request.TextualData.ContentVariances.OverlayPosterImage)
	newarr = append(newarr, request.NonTextualData.PosterImage)
	newarr = append(newarr, request.NonTextualData.DetailsBackground)
	newarr = append(newarr, request.NonTextualData.MobileDetailsBackground)
	for k := 0; k < len(newarr); k++ {
		item := newarr[k]
		filetrim := strings.Split(item, "_")
		Destination := contentId + "/" + filetrim[0]
		source := bucketName + "/" + "temp/" + item
		s, err := session.NewSession(&aws.Config{
			Region: aws.String(os.Getenv("S3_REGION")),
			Credentials: credentials.NewStaticCredentials(
				os.Getenv("S3_ID"),     // id
				os.Getenv("S3_SECRET"), // secret
				""),                    // token can be left blank for now
		})
		/* Copy object from one directory to another*/
		svc := s3.New(s)
		input := &s3.CopyObjectInput{
			Bucket:     aws.String(bucketName),
			CopySource: aws.String(source),
			Key:        aws.String(Destination),
		}
		result, err := svc.CopyObject(input)
		if err != nil {
			if aerr, ok := err.(awserr.Error); ok {
				switch aerr.Code() {
				case s3.ErrCodeObjectNotInActiveTierError:
					fmt.Println(s3.ErrCodeObjectNotInActiveTierError, aerr.Error())
				default:
					fmt.Println(aerr.Error())
				}
			} else {
				fmt.Println(err.Error())
			}
			return
		}
		fmt.Println(result, "reseult......")
		url := "https://" + bucketName + ".s3.ap-south-1.amazonaws.com/" + Destination
		// don't worry about errors
		response, e := http.Get(url)
		if e != nil {
			log.Fatal(e)
		}
		defer response.Body.Close()

		//open a file for writing
		file, err := os.Create(filetrim[0])
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		// Use io.Copy to just dump the response body to the file. This supports huge files
		_, err = io.Copy(file, response.Body)
		if err != nil {
			log.Fatal(err)
		}
		errorr := SizeUploadFileToS3(s, filetrim[0], contentId)
		if errorr != nil {
			fmt.Println("error in uploading size upload", errorr)
		}
		fmt.Println("Success!")
	}
}

/*Uploade image in S3 bucket  Based on onetier Id*/
func ContentVarianceImageUpload(Variances []Variance, contentId string) {
	bucketName := os.Getenv("S3_BUCKET")
	var newarr []string

	// newarr = append(newarr, request.TextualData.ContentVariances.OverlayPosterImage)
	for _, value := range Variances {
		newarr = append(newarr, value.OverlayPosterImage)
		newarr = append(newarr, value.DubbingScript)
		newarr = append(newarr, value.SubtitlingScript)

		for k := 0; k < len(newarr); k++ {
			item := newarr[k]
			if strings.Contains(item, "_") {
				filetrim := strings.Split(item, "_")
				Destination := contentId + "/" + value.Id + "/" + filetrim[0]
				source := bucketName + "/" + "temp/" + item

				s, err := session.NewSession(&aws.Config{
					Region: aws.String(os.Getenv("S3_REGION")),
					Credentials: credentials.NewStaticCredentials(
						os.Getenv("S3_ID"),     // id
						os.Getenv("S3_SECRET"), // secret
						""),                    // token can be left blank for now
				})
				/* Copy object from one directory to another*/
				svc := s3.New(s)
				input := &s3.CopyObjectInput{
					Bucket:     aws.String(bucketName),
					CopySource: aws.String(source),
					Key:        aws.String(Destination),
				}
				result, err := svc.CopyObject(input)
				if err != nil {
					fmt.Println("kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk")
					if aerr, ok := err.(awserr.Error); ok {
						switch aerr.Code() {
						case s3.ErrCodeObjectNotInActiveTierError:
							fmt.Println(s3.ErrCodeObjectNotInActiveTierError, aerr.Error())
						default:
							fmt.Println(aerr.Error())
						}
					} else {
						fmt.Println(err.Error())
					}
					//	return
				}
				fmt.Println(result, ";;;;;;;;;;;;;;;;;;;;;;;;;")

				url := "https://" + bucketName + ".s3.ap-south-1.amazonaws.com/" + Destination
				// don't worry about errors
				response, e := http.Get(url)
				if e != nil {
					log.Fatal(e)
				}
				defer response.Body.Close()

				//open a file for writing
				file, err := os.Create(filetrim[0])
				if err != nil {
					log.Fatal(err)
				}
				defer file.Close()

				// Use io.Copy to just dump the response body to the file. This supports huge files
				_, err = io.Copy(file, response.Body)
				if err != nil {
					log.Fatal(err)
				}
				errorr := SizeUploadFileToS3(s, filetrim[0], contentId)
				if errorr != nil {
					fmt.Println("error in uploading size upload", errorr)
				}

				fmt.Println("Success!")
			}
		}
	}

}

// SizeUploadFileToS3 saves a file to aws bucket and returns the url to the file and an error if there's any
func SizeUploadFileToS3(s *session.Session, fileName string, contentId string) error {
	// open the file for use
	file, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer file.Close()
	// get the file size and read
	// the file content into a buffer
	fileInfo, _ := file.Stat()
	var size = fileInfo.Size()
	buffer := make([]byte, size)
	file.Read(buffer)
	sizeValue := [17]string{
		"100x100/",
		"150x150/",
		"200x200/",
		"250x250/",
		"270x270/",
		"300x300/",
		"420x420/",
		"450x450/",
		"570x570/",
		"600x600/",
		"620x620/",
		"800x384/",
		"800x800/",
		"811x811/",
		"900x900/",
		"2048x670/",
		"1125x240/",
	}
	var er error
	for i := 0; i < len(sizeValue); i++ {
		s3file := sizeValue[i] + contentId + "/" + fileName
		_, er = s3.New(s).PutObject(&s3.PutObjectInput{
			Bucket:               aws.String(os.Getenv("S3_BUCKET")),
			Key:                  aws.String(s3file),
			ACL:                  aws.String("public-read"),
			Body:                 bytes.NewReader(buffer),
			ContentLength:        aws.Int64(size),
			ContentType:          aws.String(http.DetectContentType(buffer)),
			ContentDisposition:   aws.String("attachment"),
			StorageClass:         aws.String("STANDARD"),
			ServerSideEncryption: aws.String("AES256"),
		})
		if er != nil {
			fmt.Println("Unable to upload", fileName, er)
		}
		fmt.Printf("Successfully uploaded %q", fileName)
	}
	return er
}

/*Uploade image in S3 bucket  Based on variance and trailer Id*/
func ContentVarianceTrailerImageUpload(contentId string, varianceId string, varianceTrailerId string, trailerPosterImage string) {
	bucketName := os.Getenv("S3_BUCKET")
	var newarr []string

	// for _, value := range Variances.VarianceTrailer {
	newarr = append(newarr, trailerPosterImage)
	for k := 0; k < len(newarr); k++ {
		item := newarr[k]
		filetrim := strings.Split(item, "_")
		Destination := contentId + "/" + varianceId + "/" + varianceTrailerId + "/" + filetrim[0]
		source := bucketName + "/" + "temp/" + item
		s, err := session.NewSession(&aws.Config{
			Region: aws.String(os.Getenv("S3_REGION")),
			Credentials: credentials.NewStaticCredentials(
				os.Getenv("S3_ID"),     // id
				os.Getenv("S3_SECRET"), // secret
				""),                    // token can be left blank for now
		})
		/* Copy object from one directory to another*/
		svc := s3.New(s)
		input := &s3.CopyObjectInput{
			Bucket:     aws.String(bucketName),
			CopySource: aws.String(source),
			Key:        aws.String(Destination),
		}
		result, err := svc.CopyObject(input)
		if err != nil {
			if aerr, ok := err.(awserr.Error); ok {
				switch aerr.Code() {
				case s3.ErrCodeObjectNotInActiveTierError:
					fmt.Println(s3.ErrCodeObjectNotInActiveTierError, aerr.Error())
				default:
					fmt.Println(aerr.Error())
				}
			} else {
				fmt.Println(err.Error())
			}
			return
		}
		fmt.Println(result, "reseult......")
		url := "https://" + bucketName + ".s3.ap-south-1.amazonaws.com/" + Destination
		// don't worry about errors
		response, e := http.Get(url)
		if e != nil {
			log.Fatal(e)
		}
		defer response.Body.Close()

		//open a file for writing
		file, err := os.Create(filetrim[0])
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		// Use io.Copy to just dump the response body to the file. This supports huge files
		_, err = io.Copy(file, response.Body)
		if err != nil {
			log.Fatal(err)
		}
		errorr := SizeUploadFileToS3(s, filetrim[0], contentId)
		if errorr != nil {
			fmt.Println("error in uploading size upload", errorr)
		}
		fmt.Println("Success!")

	}
	// }

}
