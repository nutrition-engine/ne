package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/intervention-engine/multifactorriskservice/client"
	"github.com/intervention-engine/riskservice/plugin"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

// RegisterRoutes sets up the http request handlers with Gin
func RegisterRoutes(e *gin.Engine, fhirEndpoint, redcapEndpoint, redcapToken string, pieCollection *mgo.Collection, basisPieURL string) {
	RegisterPieHandler(e, pieCollection)
	RegisterRefreshHandler(e, fhirEndpoint, redcapEndpoint, redcapToken, pieCollection, basisPieURL)
}

// RegisterPieHandler registers the handler to return pies from the database
func RegisterPieHandler(e *gin.Engine, pieCollection *mgo.Collection) {
	e.GET("/pies/:id", func(c *gin.Context) {
		pie := &plugin.Pie{}
		id := c.Param("id")
		if bson.IsObjectIdHex(id) {
			query := pieCollection.FindId(bson.ObjectIdHex(id))
			if err := query.One(pie); err == nil {
				c.JSON(http.StatusOK, pie)
			} else {
				c.Status(http.StatusNotFound)
			}
		} else {
			c.String(http.StatusBadRequest, "Bad ID format for requested Pie. Should be a BSON Id")
		}
		return
	})
}

// RegisterRefreshHandler registers the handler to refresh risk assessments from REDCap
func RegisterRefreshHandler(e *gin.Engine, fhirEndpoint, redcapEndpoint, redcapToken string, pieCollection *mgo.Collection, basisPieURL string) {
	e.POST("/refresh", func(c *gin.Context) {
		results, err := client.RefreshRiskAssessments(fhirEndpoint, redcapEndpoint, redcapToken, pieCollection, basisPieURL)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
		client.LogResultSummary(results)
		c.JSON(http.StatusOK, results)
	})
}
