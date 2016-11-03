package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/intervention-engine/multifactorriskservice/models"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"gopkg.in/mgo.v2/dbtest"

	"github.com/gin-gonic/gin"
	fhir "github.com/intervention-engine/fhir/models"
	"github.com/intervention-engine/fhir/server"
	"github.com/intervention-engine/riskservice/plugin"
	"github.com/stretchr/testify/suite"
)

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestFHIRClientSuite(t *testing.T) {
	suite.Run(t, new(FHIRClientSuite))
}

type FHIRClientSuite struct {
	suite.Suite
	DBServer     *dbtest.DBServer
	DBServerPath string
	Session      *mgo.Session
	Database     *mgo.Database
	Server       *httptest.Server
	Studies      models.StudyMap
}

func (suite *FHIRClientSuite) SetupSuite() {
	// Turn off debug mode since all of the logging gets in the way
	gin.SetMode(gin.ReleaseMode)

	suite.DBServer = &dbtest.DBServer{}
	var err error
	suite.DBServerPath, err = ioutil.TempDir("", "mongotestdb")
	if err != nil {
		panic(err)
	}
	suite.DBServer.SetPath(suite.DBServerPath)
}

func (suite *FHIRClientSuite) SetupTest() {
	require := suite.Require()

	suite.Session = suite.DBServer.Session()
	suite.Database = suite.Session.DB("redcap-riskservice-test")

	e := gin.New()
	server.RegisterRoutes(e, nil, server.NewMongoDataAccessLayer(suite.Database), server.Config{})
	suite.Server = httptest.NewServer(e)

	// Add the patients to the database
	data, err := os.Open("../fixtures/patients_bundle.json")
	require.NoError(err)
	defer data.Close()
	res, err := http.Post(suite.Server.URL+"/", "application/json", data)
	require.NoError(err)
	defer res.Body.Close()

	// Load the studies to post
	suite.Studies = make(models.StudyMap)
	var records []models.Record
	bytes, err := ioutil.ReadFile("../fixtures/example_records.json")
	require.NoError(err)
	err = json.Unmarshal(bytes, &records)
	require.NoError(err)
	err = suite.Studies.AddRecords(records)
	require.NoError(err)
}

func (suite *FHIRClientSuite) TearDownTest() {
	suite.Server.Close()
	suite.Session.Close()
	suite.DBServer.Wipe()
}

func (suite *FHIRClientSuite) TearDownSuite() {
	suite.DBServer.Stop()
	if err := os.RemoveAll(suite.DBServerPath); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: Error cleaning up temp directory: %s", err.Error())
	}
}

func (suite *FHIRClientSuite) TestPostRiskAssessments() {
	require := suite.Require()
	assert := suite.Assert()

	// Post the studies as risk assessments
	piesCollection := suite.Database.C("pies")
	results := PostRiskAssessments(suite.Server.URL, suite.Studies, piesCollection, suite.Server.URL+"/pies")
	assert.Len(results, 2)

	// Check the results
	assert.Contains(results, Result{
		StudyID:             "1",
		FHIRPatientID:       "56fd63cdac1c5d77f6f695a1",
		RiskAssessmentCount: 2,
		Error:               nil,
	})
	assert.Contains(results, Result{
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
	count, err = piesCollection.Count()
	require.NoError(err)
	assert.Equal(count, 3)

	// Get the risk assessments
	var ras []fhir.RiskAssessment
	err = raCollection.Find(bson.M{"method.coding.code": "MultiFactor"}).Sort("date.time").All(&ras)
	require.NoError(err)

	// Check risk assessments and pies
	suite.checkRiskAssessment(&ras[0], "56fd63cdac1c5d77f6f695a1", time.Date(2015, time.December, 7, 0, 0, 0, 0, time.Local), 3, false)
	suite.checkPie(&ras[0], "56fd63cdac1c5d77f6f695a1", 3, 2, 1, 3)
	suite.checkRiskAssessment(&ras[1], "56fd63cdac1c5d77f6f695a2", time.Date(2016, time.February, 21, 0, 0, 0, 0, time.Local), 2, true)
	suite.checkPie(&ras[1], "56fd63cdac1c5d77f6f695a2", 1, 1, 2, 1)
	suite.checkRiskAssessment(&ras[2], "56fd63cdac1c5d77f6f695a1", time.Date(2016, time.April, 1, 0, 0, 0, 0, time.Local), 4, true)
	suite.checkPie(&ras[2], "56fd63cdac1c5d77f6f695a1", 3, 2, 1, 4)
}

func (suite *FHIRClientSuite) TestPostRiskAssessmentsWithUnfoundStudyID() {
	require := suite.Require()
	assert := suite.Assert()

	// Change one of the StudyIDs so it isn't found on the FHIR server
	suite.Studies["a"].ID = "FOO"
	suite.Studies["a"].Records[0].StudyID = "FOO"

	// Post the studies as risk assessments
	piesCollection := suite.Database.C("pies")
	results := PostRiskAssessments(suite.Server.URL, suite.Studies, piesCollection, suite.Server.URL+"/pies")
	assert.Len(results, 2)

	// Check the results
	assert.Contains(results, Result{
		StudyID:             "1",
		FHIRPatientID:       "56fd63cdac1c5d77f6f695a1",
		RiskAssessmentCount: 2,
		Error:               nil,
	})
	assert.Contains(results, Result{
		StudyID:             "FOO",
		FHIRPatientID:       "",
		RiskAssessmentCount: 0,
		Error:               errors.New("Couldn't find patient with Study ID FOO"),
	})

	// Check we have the right number of risk assessments
	raCollection := suite.Database.C("riskassessments")
	count, err := raCollection.Find(bson.M{"method.coding.code": "MultiFactor"}).Count()
	require.NoError(err)
	assert.Equal(count, 2)

	// Check we have the right number of pies
	count, err = piesCollection.Count()
	require.NoError(err)
	assert.Equal(count, 2)

	// Get the risk assessments
	var ras []fhir.RiskAssessment
	err = raCollection.Find(bson.M{"method.coding.code": "MultiFactor"}).Sort("date.time").All(&ras)
	require.NoError(err)

	// Check risk assessments and pies
	suite.checkRiskAssessment(&ras[0], "56fd63cdac1c5d77f6f695a1", time.Date(2015, time.December, 7, 0, 0, 0, 0, time.Local), 3, false)
	suite.checkPie(&ras[0], "56fd63cdac1c5d77f6f695a1", 3, 2, 1, 3)
	suite.checkRiskAssessment(&ras[1], "56fd63cdac1c5d77f6f695a1", time.Date(2016, time.April, 1, 0, 0, 0, 0, time.Local), 4, true)
	suite.checkPie(&ras[1], "56fd63cdac1c5d77f6f695a1", 3, 2, 1, 4)
}

func (suite *FHIRClientSuite) checkRiskAssessment(ra *fhir.RiskAssessment, patientID string, date time.Time, score int, mostRecent bool) {
	assert := suite.Assert()

	assert.Equal("Patient/"+patientID, ra.Subject.Reference)
	assert.True(ra.Method.MatchesCode("http://interventionengine.org/risk-assessments", "MultiFactor"))
	assert.True(ra.Date.Time.Equal(date))
	assert.Len(ra.Prediction, 1)
	assert.Equal("Catastrophic Health Event", ra.Prediction[0].Outcome.Text)
	assert.Equal(float64(score), *ra.Prediction[0].ProbabilityDecimal)
	assert.Len(ra.Basis, 1)
	assert.True(strings.HasPrefix(ra.Basis[0].Reference, suite.Server.URL+"/pies/"))
	if mostRecent {
		assert.Len(ra.Meta.Tag, 1)
		assert.Equal(fhir.Coding{System: "http://interventionengine.org/tags/", Code: "MOST_RECENT"}, ra.Meta.Tag[0])
	} else {
		assert.Len(ra.Meta.Tag, 0)
	}
}

func (suite *FHIRClientSuite) checkPie(ra *fhir.RiskAssessment, patientID string, clinical, functional, psychosocial, utilization int) {
	assert := suite.Assert()

	pieID := strings.TrimPrefix(ra.Basis[0].Reference, suite.Server.URL+"/pies/")
	pie := new(plugin.Pie)
	err := suite.Database.C("pies").FindId(bson.ObjectIdHex(pieID)).One(pie)
	assert.NoError(err)
	assert.Equal(suite.Server.URL+"/Patient/"+patientID, pie.Patient)
	assert.Len(pie.Slices, 4)
	assert.Equal(clinical, pie.Slices[0].Value)
	assert.Equal(functional, pie.Slices[1].Value)
	assert.Equal(psychosocial, pie.Slices[2].Value)
	assert.Equal(utilization, pie.Slices[3].Value)
}
