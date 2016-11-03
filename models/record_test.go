package models

import (
	"encoding/json"
	"io/ioutil"
	"testing"
	"time"

	"github.com/intervention-engine/riskservice/plugin"
	"github.com/stretchr/testify/suite"
)

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestRecordSuite(t *testing.T) {
	suite.Run(t, new(RecordSuite))
}

type RecordSuite struct {
	suite.Suite
	Records []Record
}

func (suite *RecordSuite) SetupTest() {
	require := suite.Require()

	data, err := ioutil.ReadFile("../fixtures/example_records.json")
	require.NoError(err)
	err = json.Unmarshal(data, &suite.Records)
	require.NoError(err)
}

func (suite *RecordSuite) TestLoadRecordsFromJSON() {
	assert := suite.Assert()
	assert.Len(suite.Records, 3)
	assert.Equal(Record{
		StudyID:             float64(1),
		EventName:           "initial_arm_1",
		RiskFactorDate:      "2015-12-07",
		ClinicalRisk:        "3",
		FunctionalRisk:      "2",
		PsychosocialRisk:    "1",
		UtilizationRisk:     "3",
		PerceivedRisk:       "3",
	}, suite.Records[0])
	assert.Equal(Record{
		StudyID:             float64(1),
		EventName:           "visit1_arm_1",
		RiskFactorDate:      "2016-04-01",
		ClinicalRisk:        "3",
		FunctionalRisk:      "2",
		PsychosocialRisk:    "1",
		UtilizationRisk:     "4",
		PerceivedRisk:       "4",
	}, suite.Records[1])
	assert.Equal(Record{
		StudyID:             "a",
		EventName:           "initial_arm_1",
		RiskFactorDate:      "2016-02-21",
		ClinicalRisk:        "1",
		FunctionalRisk:      "1",
		PsychosocialRisk:    "2",
		UtilizationRisk:     "1",
		PerceivedRisk:       "2",
	}, suite.Records[2])
}

func (suite *RecordSuite) TestStudyIDString() {
	assert := suite.Assert()
	assert.Equal("1", suite.Records[0].StudyIDString())
	assert.Equal("1", suite.Records[1].StudyIDString())
	assert.Equal("a", suite.Records[2].StudyIDString())
}

func (suite *RecordSuite) TestRiskFactorDateTime() {
	assert := suite.Assert()
	t, e := suite.Records[0].RiskFactorDateTime()
	assert.NoError(e)
	assert.Equal(time.Date(2015, time.December, 7, 0, 0, 0, 0, time.Local), t)

	t, e = suite.Records[1].RiskFactorDateTime()
	assert.NoError(e)
	assert.Equal(time.Date(2016, time.April, 1, 0, 0, 0, 0, time.Local), t)

	t, e = suite.Records[2].RiskFactorDateTime()
	assert.NoError(e)
	assert.Equal(time.Date(2016, time.February, 21, 0, 0, 0, 0, time.Local), t)
}

func (suite *RecordSuite) TestIsRiskFactorsComplete() {
	assert := suite.Assert()
	record := suite.Records[0]
	assert.True(record.IsRiskFactorsComplete())

	record = suite.Records[0]
	record.ClinicalRisk = ""
	assert.False(record.IsRiskFactorsComplete(), "Empty ClinicalRisk flag indicates NOT complete")

	record = suite.Records[0]
	record.FunctionalRisk = ""
	assert.False(record.IsRiskFactorsComplete(), "Empty FunctionalRisk flag indicates NOT complete")

	record = suite.Records[0]
	record.PsychosocialRisk = ""
	assert.False(record.IsRiskFactorsComplete(), "Empty PsychosocialRisk flag indicates NOT complete")

	record = suite.Records[0]
	record.UtilizationRisk = ""
	assert.False(record.IsRiskFactorsComplete(), "Empty UtilizationRisk flag indicates NOT complete")

	record = suite.Records[0]
	record.PerceivedRisk = ""
	assert.False(record.IsRiskFactorsComplete(), "Empty PerceivedRisk flag indicates NOT complete")
}

func (suite *RecordSuite) TestToPie() {
	pie, err := suite.Records[0].ToPie("http://fhir/Patient/1")
	suite.Require().NoError(err)
	suite.assertPieForRecord0(pie)
}

func (suite *RecordSuite) TestIncompleteRiskFactorsToPie() {
	assert := suite.Assert()

	record := suite.Records[0]
	record.ClinicalRisk = ""
	pie, err := record.ToPie("http://fhir/Patient/1")
	assert.Nil(pie)
	assert.Error(err)
}

func (suite *RecordSuite) TestToRiskServiceCalculationResult() {
	assert := suite.Assert()
	require := suite.Require()

	result, err := suite.Records[0].ToRiskServiceCalculationResult("http://fhir/Patient/1")
	require.NoError(err)
	require.NotNil(result)
	assert.Equal(time.Date(2015, time.December, 7, 0, 0, 0, 0, time.Local), result.AsOf)
	assert.Equal(3, *result.Score)
	assert.Nil(result.ProbabilityDecimal)
	suite.assertPieForRecord0(result.Pie)
}

func (suite *RecordSuite) assertPieForRecord0(pie *plugin.Pie) {
	assert := suite.Assert()
	require := suite.Require()

	require.NotNil(pie)
	assert.True(!pie.Created.IsZero(), "Created time should not be zero time")
	assert.NotEmpty(pie.Id.Hex())
	assert.Equal(pie.Patient, "http://fhir/Patient/1")
	require.Len(pie.Slices, 4)
	for _, slice := range pie.Slices {
		assert.Equal(25, slice.Weight)
		assert.Equal(4, slice.MaxValue)
	}
	assert.Equal("Clinical Risk", pie.Slices[0].Name)
	assert.Equal(3, pie.Slices[0].Value)
	assert.Equal("Functional and Environmental Risk", pie.Slices[1].Name)
	assert.Equal(2, pie.Slices[1].Value)
	assert.Equal("Psychosocial and Mental Health Risk", pie.Slices[2].Name)
	assert.Equal(1, pie.Slices[2].Value)
	assert.Equal("Utilization Risk", pie.Slices[3].Name)
	assert.Equal(3, pie.Slices[3].Value)
}
