package server

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/intervention-engine/multifactorriskservice/models"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"gopkg.in/mgo.v2/dbtest"

	"github.com/gin-gonic/gin"
	"github.com/intervention-engine/fhir/server"
	"github.com/robfig/cron"
	"github.com/stretchr/testify/suite"
)

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestCronSuite(t *testing.T) {
	suite.Run(t, new(CronSuite))
}

type CronSuite struct {
	suite.Suite
	DBServer     *dbtest.DBServer
	DBServerPath string
	Session      *mgo.Session
	Database     *mgo.Database
	FHIRServer   *httptest.Server
	REDCapServer *httptest.Server
	Studies      models.StudyMap
}

func (suite *CronSuite) SetupSuite() {
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

func (suite *CronSuite) SetupTest() {
	require := suite.Require()

	suite.Session = suite.DBServer.Session()
	suite.Database = suite.Session.DB("redcap-riskservice-test")

	fe := gin.New()
	server.RegisterRoutes(fe, nil, server.NewMongoDataAccessLayer(suite.Database), server.Config{})
	suite.FHIRServer = httptest.NewServer(fe)

	// Add the patients to the database
	data, err := os.Open("../fixtures/patients_bundle.json")
	require.NoError(err)
	defer data.Close()
	res, err := http.Post(suite.FHIRServer.URL+"/", "application/json", data)
	require.NoError(err)
	defer res.Body.Close()
}

func (suite *CronSuite) TearDownTest() {
	if suite.FHIRServer != nil {
		suite.FHIRServer.Close()
	}
	suite.Session.Close()
	suite.DBServer.Wipe()
}

func (suite *CronSuite) TearDownSuite() {
	suite.DBServer.Stop()
	if err := os.RemoveAll(suite.DBServerPath); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: Error cleaning up temp directory: %s", err.Error())
	}
	if suite.REDCapServer != nil {
		suite.REDCapServer.Close()
	}
}

// Warning: This test *should* be OK -- but because it relies on timing, it *could* get flakey
func (suite *CronSuite) TestCron() {
	require := suite.Require()
	assert := suite.Assert()

	// Schedule the cron
	c := cron.New()
	err := ScheduleRefreshRiskAssessmentsCron(c, "@every 1s", suite.FHIRServer.URL, suite.REDCapServer.URL, "12345", suite.Database.C("pies"), "http://example.org/pies/")
	c.Start()
	defer c.Stop()

	// Check we have no risk assessments yet
	raCollection := suite.Database.C("riskassessments")
	count, err := raCollection.Find(bson.M{"method.coding.code": "MultiFactor"}).Count()
	require.NoError(err)
	assert.Equal(0, count)

	// Check for updated count every 500 ms for a total of 10s, then give up.
	// This helps account for slow machines.
	for i := 0; i < 20 && count != 3; i++ {
		time.Sleep(500 * time.Millisecond)
		count, err = raCollection.Find(bson.M{"method.coding.code": "MultiFactor"}).Count()
		require.NoError(err)
	}
	assert.Equal(3, count)
}
