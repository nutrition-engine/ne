package models

import (
	"encoding/json"
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestStudySuite(t *testing.T) {
	suite.Run(t, new(StudySuite))
}

type StudySuite struct {
	suite.Suite
	Records []Record
}

func (suite *StudySuite) SetupTest() {
	require := suite.Require()

	data, err := ioutil.ReadFile("../fixtures/example_records.json")
	require.NoError(err)
	err = json.Unmarshal(data, &suite.Records)
	require.NoError(err)
}

func (suite *StudySuite) TestAddOneRecord() {
	assert := suite.Assert()

	study := new(Study)

	// Add first record
	err := study.AddRecord(suite.Records[0])
	assert.NoError(err)
	assert.Equal("1", study.ID)
	assert.Len(study.Records, 1)
	assert.Equal(suite.Records[0], study.Records[0])
}

func (suite *StudySuite) TestTwoRecords() {
	assert := suite.Assert()

	study := new(Study)

	// Add first record
	err := study.AddRecord(suite.Records[0])
	assert.NoError(err)

	// Add second record
	second := suite.Records[1]
	err = study.AddRecord(second)
	assert.NoError(err)
}

func (suite *StudySuite) TestAddSecondRecordWithNonMatchingStudyID() {
	assert := suite.Assert()

	study := new(Study)

	// Add first record
	err := study.AddRecord(suite.Records[0])
	assert.NoError(err)

	// Add second record
	second := suite.Records[1]
	second.StudyID = 2
	err = study.AddRecord(second)
	assert.Error(err)
}

func (suite *StudySuite) TestToRiskServiceCalculationResults() {
	assert := suite.Assert()
	require := suite.Require()

	study := new(Study)
	study.AddRecord(suite.Records[0])
	study.AddRecord(suite.Records[1])
	results := study.ToRiskServiceCalculationResults("http://fhir/Patient/1")

	require.Len(results, 2)
	assert.Equal(time.Date(2015, time.December, 7, 0, 0, 0, 0, time.Local), results[0].AsOf)
	assert.Equal(3, *results[0].Score)
	assert.Nil(results[0].ProbabilityDecimal)
	assert.NotNil(results[0].Pie)
	assert.Equal(results[0].Pie.Patient, "http://fhir/Patient/1")
	assert.Equal(time.Date(2016, time.April, 1, 0, 0, 0, 0, time.Local), results[1].AsOf)
	assert.Equal(4, *results[1].Score)
	assert.Nil(results[1].ProbabilityDecimal)
	assert.NotNil(results[1].Pie)
	assert.Equal(results[1].Pie.Patient, "http://fhir/Patient/1")
}

func (suite *StudySuite) TestToRiskServiceCalculationResultsSortsByDate() {
	assert := suite.Assert()
	require := suite.Require()

	study := new(Study)
	study.AddRecord(suite.Records[1])
	study.AddRecord(suite.Records[0])
	results := study.ToRiskServiceCalculationResults("http://fhir/Patient/1")

	require.Len(results, 2)
	assert.Equal(time.Date(2015, time.December, 7, 0, 0, 0, 0, time.Local), results[0].AsOf)
	assert.Equal(3, *results[0].Score)
	assert.Nil(results[0].ProbabilityDecimal)
	assert.NotNil(results[0].Pie)
	assert.Equal(results[0].Pie.Patient, "http://fhir/Patient/1")
	assert.Equal(time.Date(2016, time.April, 1, 0, 0, 0, 0, time.Local), results[1].AsOf)
	assert.Equal(4, *results[1].Score)
	assert.Nil(results[1].ProbabilityDecimal)
	assert.NotNil(results[1].Pie)
	assert.Equal(results[1].Pie.Patient, "http://fhir/Patient/1")
}

func (suite *StudySuite) TestToRiskServiceCalculationResultsIgnoresIncompletes() {
	assert := suite.Assert()
	require := suite.Require()

	study := new(Study)
	study.AddRecord(suite.Records[0])
	incomplete := suite.Records[1]
	incomplete.FunctionalRisk = ""
	study.AddRecord(incomplete)
	assert.Len(study.Records, 2)
	results := study.ToRiskServiceCalculationResults("http://fhir/Patient/1")

	require.Len(results, 1)
	assert.Equal(time.Date(2015, time.December, 7, 0, 0, 0, 0, time.Local), results[0].AsOf)
	assert.Equal(3, *results[0].Score)
	assert.Nil(results[0].ProbabilityDecimal)
	assert.NotNil(results[0].Pie)
	assert.Equal(results[0].Pie.Patient, "http://fhir/Patient/1")
}

func (suite *StudySuite) TestStudyMapAddRecord() {
	assert := suite.Assert()
	require := suite.Require()

	m := make(StudyMap)
	m.AddRecord(suite.Records[0])
	require.Len(m, 1)

	s, ok := m["1"]
	require.True(ok)
	assert.Equal("1", s.ID)
	require.Len(s.Records, 1)
	assert.Equal(suite.Records[0], s.Records[0])

	s, ok = m["2"]
	assert.False(ok)
}

func (suite *StudySuite) TestStudyMapAddRecords() {
	assert := suite.Assert()
	require := suite.Require()

	m := make(StudyMap)
	m.AddRecords(suite.Records)
	require.Len(m, 2)

	s, ok := m["1"]
	require.True(ok)
	assert.Equal("1", s.ID)
	require.Len(s.Records, 2)
	assert.Equal(suite.Records[0], s.Records[0])
	assert.Equal(suite.Records[1], s.Records[1])

	s, ok = m["a"]
	require.True(ok)
	assert.Equal("a", s.ID)
	require.Len(s.Records, 1)
	assert.Equal(suite.Records[2], s.Records[0])
}
