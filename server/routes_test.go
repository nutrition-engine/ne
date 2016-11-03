package server

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/intervention-engine/multifactorriskservice/client"
	"github.com/intervention-engine/multifactorriskservice/models"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"gopkg.in/mgo.v2/dbtest"

	"github.com/gin-gonic/gin"
	"github.com/intervention-engine/fhir/server"
	"github.com/intervention-engine/riskservice/plugin"
	"github.com/stretchr/testify/suite"
)

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestRoutesSuite(t *testing.T) {
	suite.Run(t, new(RoutesSuite))
}

type RoutesSuite struct {
	suite.Suite
	DBServer     *dbtest.DBServer
	DBServerPath string
	Session      *mgo.Session
	Database     *mgo.Database
	Server       *httptest.Server
	FHIRServer   *httptest.Server
	REDCapServer *httptest.Server
	Studies      models.StudyMap
}

func (suite *RoutesSuite) SetupSuite() {
	require := suite.Require()

	// Turn off debug mode since all of the logging gets in the way
	gin.SetMode(gin.ReleaseMode)

	suite.DBServer = &dbtest.DBServer{}
	var err error
	suite.DBServerPath, err = ioutil.TempDir("", "mongotestdb")
	if err != nil {
		panic(err)
	}
	suite.DBServer.SetPath(suite.DBServerPath)

	// Setup the mock REDCap server
	f, err := os.Open("../fixtures/example_records.json")
	require.NoError(err)
	suite.REDCapServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		io.Copy(w, f)
	}))
}

func (suite *RoutesSuite) SetupTest() {
	suite.Session = suite.DBServer.Session()
	suite.Database = suite.Session.DB("redcap-riskservice-test")

	fe := gin.New()
	server.RegisterRoutes(fe, nil, server.NewMongoDataAccessLayer(suite.Database), server.Config{})
	suite.FHIRServer = httptest.NewServer(fe)

	e := gin.New()
	suite.Server = httptest.NewServer(e)
	RegisterRoutes(e, suite.FHIRServer.URL, suite.REDCapServer.URL, "123abc", suite.Database.C("pies"), suite.Server.URL+"/pies/")
}

func (suite *RoutesSuite) TearDownTest() {
	suite.FHIRServer.Close()
	suite.Server.Close()
	suite.Session.Close()
	suite.DBServer.Wipe()
}

func (suite *RoutesSuite) TearDownSuite() {
	suite.DBServer.Stop()
	if err := os.RemoveAll(suite.DBServerPath); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: Error cleaning up temp directory: %s", err.Error())
	}
	if suite.REDCapServer != nil {
		suite.REDCapServer.Close()
	}
}

func (suite *RoutesSuite) TestRefresh() {
	require := suite.Require()
	assert := suite.Assert()

	// Add the patients to the database
	data, err := os.Open("../fixtures/patients_bundle.json")
	require.NoError(err)
	defer data.Close()
	res, err := http.Post(suite.FHIRServer.URL+"/", "application/json", data)
	require.NoError(err)
	defer res.Body.Close()

	// Trigger the refresh
	res, err = http.DefaultClient.Post(suite.Server.URL+"/refresh", "application/json", nil)
	require.NoError(err)
	defer res.Body.Close()
	assert.Equal(http.StatusOK, res.StatusCode)
	var results []client.Result
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(&results)
	require.NoError(err)

	// Check the results
	assert.Len(results, 2)
	assert.Contains(results, client.Result{
		StudyID:             "1",
		FHIRPatientID:       "56fd63cdac1c5d77f6f695a1",
		RiskAssessmentCount: 2,
		Error:               nil,
	})
	assert.Contains(results, client.Result{
		StudyID:             "a",
		FHIRPatientID:       "56fd63cdac1c5d77f6f695a2",
		RiskAssessmentCount: 1,
		Error:               nil,
	})

	// Check we have the right number of risk assessments
	raCollection := suite.Database.C("riskassessments")
	count, err := raCollection.Find(bson.M{"method.coding.code": "MultiFactor"}).Count()
	require.NoError(err)
	assert.Equal(count, 3)

	// Check we have the right number of pies
	piesCollection := suite.Database.C("pies")
	count, err = piesCollection.Count()
	require.NoError(err)
	assert.Equal(count, 3)
}

func (suite *RoutesSuite) TestGetPie() {
	require := suite.Require()
	assert := suite.Assert()

	// Create and store the pie
	pie := new(plugin.Pie)
	pie.Id = bson.NewObjectId()
	pieTime := time.Now().Truncate(time.Millisecond) // Truncate since mongo doesn't support nanoseconds
	pie.Created = pieTime
	pie.Patient = suite.FHIRServer.URL + "/Patient/56fd63cdac1c5d77f6f695a1"
	pie.Slices = make([]plugin.Slice, 4)
	copy(pie.Slices, client.REDCapRiskServiceConfig.DefaultPieSlices)
	pie.Slices[0].Value = 1
	pie.Slices[1].Value = 2
	pie.Slices[2].Value = 3
	pie.Slices[3].Value = 4

	piesCollection := suite.Database.C("pies")
	err := piesCollection.Insert(pie)
	require.NoError(err)

	// Get the pie
	res, err := http.DefaultClient.Get(suite.Server.URL + "/pies/" + pie.Id.Hex())
	require.NoError(err)
	defer res.Body.Close()
	assert.Equal(http.StatusOK, res.StatusCode)
	pie2 := new(plugin.Pie)
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(&pie2)
	require.NoError(err)

	// The date can cause issues due to location/timezone, so check that first and then set them equal for the full
	// pie comparison (I think this was causing failures on travis)
	if assert.True(pie.Created.Equal(pie2.Created)) {
		pie2.Created = pie.Created
	}
	assert.Equal(pie, pie2)
}

func (suite *RoutesSuite) TestGetInvalidPie() {
	require := suite.Require()
	assert := suite.Assert()

	// Get some pie that doesn't exist
	res, err := http.DefaultClient.Get(suite.Server.URL + "/pies/" + bson.NewObjectId().Hex())
	require.NoError(err)
	defer res.Body.Close()
	assert.Equal(http.StatusNotFound, res.StatusCode)
}
