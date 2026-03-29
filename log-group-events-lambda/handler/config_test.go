package handler

import (
	"fmt"
	"os"
	"testing"

	"github.com/logzio/firehose-logs/common"
	"github.com/logzio/firehose-logs/logger"
	"github.com/stretchr/testify/assert"
)

func InitConfigTest() {
	sugLog = logger.GetSugaredLogger()
}

func TestNewConfigMissingEnv(t *testing.T) {
	InitConfigTest()
	conf := NewConfig()
	assert.Nil(t, conf)
}

func TestNewConfigValidRequired(t *testing.T) {
	InitConfigTest()

	/* Setup required env variable */
	err := os.Setenv(envFirehoseArn, "test-arn")
	if err != nil {
		return
	}

	err = os.Setenv(envAccountId, "aws-account-id")
	if err != nil {
		return
	}

	err = os.Setenv(envAwsPartition, "test-partition")
	if err != nil {
		return
	}

	conf := NewConfig()
	assert.NotNil(t, conf)
	assert.Equal(t, "test-arn", conf.destinationArn)
	assert.Equal(t, "aws-account-id", conf.accountId)
	assert.Equal(t, "", conf.region)
	assert.Equal(t, "/aws/lambda/", conf.thisFunctionLogGroup)
	assert.Equal(t, "", conf.thisFunctionName)
	assert.Equal(t, "", conf.customGroupsValue)
	assert.Equal(t, "", conf.servicesValue)
}

func TestNewConfigValid(t *testing.T) {
	InitConfigTest()

	/* Setup env variable */
	err := os.Setenv(envFirehoseArn, "test-arn")
	if err != nil {
		return
	}

	err = os.Setenv(envAccountId, "aws-account-id")
	if err != nil {
		return
	}

	err = os.Setenv(envAwsPartition, "test-partition")
	if err != nil {
		return
	}

	err = os.Setenv(envFunctionName, "g2")
	if err != nil {
		return
	}

	fmt.Println("test3")

	conf := NewConfig()
	assert.NotNil(t, conf)
	assert.Equal(t, "test-arn", conf.destinationArn)
	assert.Equal(t, "aws-account-id", conf.accountId)
	assert.Equal(t, "/aws/lambda/g2", conf.thisFunctionLogGroup)
	assert.Equal(t, "g2", conf.thisFunctionName)
	assert.Equal(t, "", conf.customGroupsValue)
	assert.Equal(t, "", conf.servicesValue)
	assert.Equal(t, "", conf.region)
	// Test default monitoring tag values
	assert.Equal(t, "logzio:logs", conf.monitoringTagKey)
	assert.Equal(t, "true", conf.monitoringTagValue)
}

func TestNewConfigWithCustomMonitoringTag(t *testing.T) {
	InitConfigTest()

	err := os.Setenv(envFirehoseArn, "test-arn")
	assert.Nil(t, err)
	err = os.Setenv(envAccountId, "aws-account-id")
	assert.Nil(t, err)
	err = os.Setenv(envAwsPartition, "test-partition")
	assert.Nil(t, err)
	err = os.Setenv(envMonitoringTagKey, "CustomTag")
	assert.Nil(t, err)
	err = os.Setenv(envMonitoringTagValue, "enabled")
	assert.Nil(t, err)

	conf := NewConfig()
	assert.NotNil(t, conf)
	
	// Test custom monitoring tag values
	assert.Equal(t, "CustomTag", conf.monitoringTagKey)
	assert.Equal(t, "enabled", conf.monitoringTagValue)
	
	// Clean up
	os.Unsetenv(envMonitoringTagKey)
	os.Unsetenv(envMonitoringTagValue)
}

func TestNewConfigTagEventsEnabled(t *testing.T) {
	InitConfigTest()

	tests := []struct {
		name             string
		envValue         string
		expectedEnabled  bool
	}{
		{
			name:            "TAG_EVENTS_ENABLED not set (default)",
			envValue:        "",
			expectedEnabled: false,
		},
		{
			name:            "TAG_EVENTS_ENABLED set to true",
			envValue:        "true",
			expectedEnabled: true,
		},
		{
			name:            "TAG_EVENTS_ENABLED set to True",
			envValue:        "True",
			expectedEnabled: true,
		},
		{
			name:            "TAG_EVENTS_ENABLED set to TRUE",
			envValue:        "TRUE",
			expectedEnabled: true,
		},
		{
			name:            "TAG_EVENTS_ENABLED set to false",
			envValue:        "false",
			expectedEnabled: false,
		},
		{
			name:            "TAG_EVENTS_ENABLED set to invalid value",
			envValue:        "yes",
			expectedEnabled: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Setup required env variables
			err := os.Setenv(envFirehoseArn, "test-arn")
			assert.Nil(t, err)
			err = os.Setenv(envAccountId, "aws-account-id")
			assert.Nil(t, err)
			err = os.Setenv(envAwsPartition, "test-partition")
			assert.Nil(t, err)

			if test.envValue != "" {
				err = os.Setenv(envTagEventsEnabled, test.envValue)
				assert.Nil(t, err)
			} else {
				os.Unsetenv(envTagEventsEnabled)
			}

			conf := NewConfig()
			assert.NotNil(t, conf)
			assert.Equal(t, test.expectedEnabled, conf.tagEventsEnabled)

			// Clean up
			os.Unsetenv(envTagEventsEnabled)
		})
	}
}

func TestValidateRequired(t *testing.T) {
	/* Setup tests */
	InitConfigTest()

	tests := []struct {
		name          string
		conf          Config
		expectedError bool
		errorStr      string
	}{
		{
			name: "missing all 3 required",
			conf: Config{
				awsPartition:   "",
				destinationArn: "",
				accountId:      "",
			},
			expectedError: true,
			errorStr:      "destination ARN must be set",
		},
		{
			name: "missing 2 required",
			conf: Config{
				awsPartition:   "partition",
				destinationArn: "",
				accountId:      "",
			},
			expectedError: true,
			errorStr:      "destination ARN must be set",
		},
		{
			name: "missing 1 required",
			conf: Config{
				awsPartition:   "partition",
				destinationArn: "some-arn",
				accountId:      "",
			},
			expectedError: true,
			errorStr:      "account id must be set",
		},
		{
			name: "valid",
			conf: Config{
				awsPartition:   "partition",
				destinationArn: "some-arn",
				accountId:      "accountId",
			},
			expectedError: false,
			errorStr:      "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := test.conf.validateRequired()
			if test.expectedError {
				assert.NotNil(t, result)
				assert.Equal(t, test.errorStr, result.Error())
			} else {
				assert.Nil(t, result)
			}
		})

	}
}

func TestValidateFilterPattern(t *testing.T) {
	InitConfigTest()

	err := os.Setenv(common.EnvAwsRegion, "us-east-1")
	if err != nil {
		t.Fatalf("Failed to set AWS region: %v", err)
	}

	tests := []struct {
		name          string
		filterPattern string
		expectedError bool
		errorContains string
	}{
		{
			name:          "empty pattern",
			filterPattern: "",
			expectedError: false,
			errorContains: "",
		},
		{
			name:          "valid simple pattern",
			filterPattern: "[ip, user_id, username]",
			expectedError: false,
			errorContains: "",
		},
		{
			name:          "valid pattern with wildcard",
			filterPattern: "[..., status_code=404, size]",
			expectedError: false,
			errorContains: "",
		},
		{
			name:          "valid text pattern",
			filterPattern: "\"ERROR\"",
			expectedError: false,
			errorContains: "",
		},
		{
			name:          "valid complex pattern",
			filterPattern: "[timestamp, request_id, level=ERROR, message]",
			expectedError: false,
			errorContains: "",
		},
		{
			name:          "unbalanced brackets",
			filterPattern: "[ip, user_id",
			expectedError: true,
			errorContains: "invalid filter pattern",
		},
		{
			name:          "invalid syntax",
			filterPattern: "[timestamp=, request_id, level=ERROR]",
			expectedError: true,
			errorContains: "invalid filter pattern",
		},
		{
			name:          "unbalanced quotes",
			filterPattern: "\"ERROR",
			expectedError: true,
			errorContains: "invalid filter pattern",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := &Config{
				region:        "us-east-1",
				filterPattern: test.filterPattern,
			}

			err := config.validateFilterPattern()

			if test.expectedError {
				assert.Error(t, err)
				if test.errorContains != "" {
					assert.Contains(t, err.Error(), test.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
