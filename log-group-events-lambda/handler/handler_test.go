package handler

import (
	"context"
	"os"
	"testing"

	"github.com/logzio/firehose-logs/common"
	"github.com/logzio/firehose-logs/logger"
	"github.com/stretchr/testify/assert"
)

func setupHandlerTest() (ctx context.Context) {
	/* Setup needed env variables */
	err := os.Setenv("FIREHOSE_ARN", "test-arn")
	if err != nil {
		return
	}
	err = os.Setenv("ACCOUNT_ID", "aws-account-id")
	if err != nil {
		return
	}
	err = os.Setenv("AWS_PARTITION", "test-partition")
	if err != nil {
		return
	}

	ctx = context.Background()

	return ctx
}

func TestUnsupportedEventHandling(t *testing.T) {
	ctx := setupHandlerTest()

	tests := []struct {
		name              string
		event             map[string]interface{}
		expectedOutputMsg string
		expectedError     bool
	}{
		{
			name: "Unsupported event with all required fields",
			event: map[string]interface{}{
				"detail": map[string]interface{}{
					"eventName": "MyCustomEvent",
					"requestParameters": map[string]interface{}{
						"logGroupName": "my-log-group",
					},
				},
			},
			expectedOutputMsg: "",
			expectedError:     true,
		},
		{
			name:              "Unsupported event with missing detail field",
			event:             map[string]interface{}{},
			expectedOutputMsg: "",
			expectedError:     true,
		},
		{
			name: "Unsupported event with missing eventName field",
			event: map[string]interface{}{
				"detail": map[string]interface{}{
					"requestParameters": map[string]interface{}{
						"logGroupName": "my-log-group",
					},
				},
			},
			expectedOutputMsg: "",
			expectedError:     true,
		},
		{
			name: "Unsupported event with missing requestParameters field",
			event: map[string]interface{}{
				"detail": map[string]interface{}{
					"eventName": "MyCustomEvent",
				},
			},
			expectedOutputMsg: "",
			expectedError:     true,
		},
		{
			name: "CreateLogGroup event with missing logGroup field",
			event: map[string]interface{}{
				"detail": map[string]interface{}{
					"eventName":         "CreateLogGroup",
					"requestParameters": map[string]interface{}{},
				},
			},
			expectedOutputMsg: "",
			expectedError:     true,
		},
		{
			name: "PutSecretValue event with missing secretId field",
			event: map[string]interface{}{
				"detail": map[string]interface{}{
					"eventName":         "PutSecretValue",
					"requestParameters": map[string]interface{}{},
				},
			},
			expectedOutputMsg: "",
			expectedError:     true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res, err := HandleRequest(ctx, test.event)

			assert.NotNil(t, err)
			assert.Equal(t, test.expectedOutputMsg, res)
		})
	}
}

func TestTagResourceEventHandling(t *testing.T) {
	ctx := setupHandlerTest()

	// Enable tag events for this test
	err := os.Setenv("TAG_EVENTS_ENABLED", "true")
	assert.Nil(t, err)
	defer os.Unsetenv("TAG_EVENTS_ENABLED")

	tests := []struct {
		name              string
		event             map[string]interface{}
		expectedOutputMsg string
		expectedError     bool
	}{
		{
			name: "TagResource event for CloudWatch Log Group with valid ARN",
			event: map[string]interface{}{
				"detail": map[string]interface{}{
					"eventName": "TagResource",
					"requestParameters": map[string]interface{}{
						"resourceArn": "arn:aws:logs:us-east-1:123456789012:log-group:/aws/lambda/my-function",
						"tags": map[string]interface{}{
							"logzio:logs": "true",
						},
					},
				},
			},
			expectedOutputMsg: "TagResource event handled successfully",
			expectedError:     false,
		},
		{
			name: "TagResource event with missing resourceArn field",
			event: map[string]interface{}{
				"detail": map[string]interface{}{
					"eventName": "TagResource",
					"requestParameters": map[string]interface{}{
						"tags": map[string]interface{}{
							"logzio:logs": "true",
						},
					},
				},
			},
			expectedOutputMsg: "",
			expectedError:     true,
		},
		{
			name: "TagResource event with invalid ARN format",
			event: map[string]interface{}{
				"detail": map[string]interface{}{
					"eventName": "TagResource",
					"requestParameters": map[string]interface{}{
						"resourceArn": "not-an-arn",
						"tags": map[string]interface{}{
							"logzio:logs": "true",
						},
					},
				},
			},
			expectedOutputMsg: "",
			expectedError:     true,
		},
		{
			name: "TagResource20170331v2 event for Lambda with valid ARN",
			event: map[string]interface{}{
				"detail": map[string]interface{}{
					"eventName": "TagResource20170331v2",
					"requestParameters": map[string]interface{}{
						"resource": "arn:aws:lambda:us-east-1:123456789012:function:my-payment-processor",
						"tags": map[string]interface{}{
							"logzio:logs": "true",
						},
					},
				},
			},
			expectedOutputMsg: "TagResource20170331v2 event handled successfully",
			expectedError:     false,
		},
		{
			name: "TagResource20170331v2 event with missing resource field",
			event: map[string]interface{}{
				"detail": map[string]interface{}{
					"eventName": "TagResource20170331v2",
					"requestParameters": map[string]interface{}{
						"tags": map[string]interface{}{
							"logzio:logs": "true",
						},
					},
				},
			},
			expectedOutputMsg: "",
			expectedError:     true,
		},
		{
			name: "TagResource20170331v2 event with invalid Lambda ARN",
			event: map[string]interface{}{
				"detail": map[string]interface{}{
					"eventName": "TagResource20170331v2",
					"requestParameters": map[string]interface{}{
						"resource": "invalid-lambda-arn",
						"tags": map[string]interface{}{
							"logzio:logs": "true",
						},
					},
				},
			},
			expectedOutputMsg: "",
			expectedError:     true,
		},
		{
			name: "TagResource event for Log Group with colon in name",
			event: map[string]interface{}{
				"detail": map[string]interface{}{
					"eventName": "TagResource",
					"requestParameters": map[string]interface{}{
						"resourceArn": "arn:aws:logs:us-east-1:123456789012:log-group:/aws/lambda/my-function:with:colons",
						"tags": map[string]interface{}{
							"logzio:logs": "true",
						},
					},
				},
			},
			expectedOutputMsg: "TagResource event handled successfully",
			expectedError:     false,
		},
		{
			name: "TagResource20170331v2 event for Lambda with colon in function name",
			event: map[string]interface{}{
				"detail": map[string]interface{}{
					"eventName": "TagResource20170331v2",
					"requestParameters": map[string]interface{}{
						"resource": "arn:aws:lambda:us-east-1:123456789012:function:my-function:alias",
						"tags": map[string]interface{}{
							"logzio:logs": "true",
						},
					},
				},
			},
			expectedOutputMsg: "TagResource20170331v2 event handled successfully",
			expectedError:     false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res, err := HandleRequest(ctx, test.event)

			if test.expectedError {
				assert.NotNil(t, err)
				assert.Equal(t, test.expectedOutputMsg, res)
			} else {
				assert.Nil(t, err)
				assert.Equal(t, test.expectedOutputMsg, res)
			}
		})
	}
}

func TestHasMonitoringTag(t *testing.T) {
	// Initialize logger and config (normally done in HandleRequest)
	sugLog = logger.GetSugaredLogger()
	envConfig = NewConfig()

	tests := []struct {
		name              string
		requestParameters map[string]interface{}
		expectedResult    bool
	}{
		{
			name: "Default monitoring tag logzio:logs with true value (lowercase)",
			requestParameters: map[string]interface{}{
				"tags": map[string]interface{}{
					"logzio:logs": "true",
				},
			},
			expectedResult: true,
		},
		{
			name: "Default monitoring tag with True value (capitalized)",
			requestParameters: map[string]interface{}{
				"tags": map[string]interface{}{
					"logzio:logs": "True",
				},
			},
			expectedResult: true,
		},
		{
			name: "Default monitoring tag with TRUE value (all uppercase)",
			requestParameters: map[string]interface{}{
				"tags": map[string]interface{}{
					"LOGZIO:LOGS": "TRUE",
				},
			},
			expectedResult: true,
		},
		{
			name: "Default monitoring tag with mixed case",
			requestParameters: map[string]interface{}{
				"tags": map[string]interface{}{
					"LogzIO:Logs": "TrUe",
				},
			},
			expectedResult: true,
		},
		{
			name: "Monitoring tag with False value",
			requestParameters: map[string]interface{}{
				"tags": map[string]interface{}{
					"logzio:logs": "False",
				},
			},
			expectedResult: false,
		},
		{
			name: "Monitoring tag with empty value",
			requestParameters: map[string]interface{}{
				"tags": map[string]interface{}{
					"logzio:logs": "",
				},
			},
			expectedResult: false,
		},
		{
			name: "Different tag present",
			requestParameters: map[string]interface{}{
				"tags": map[string]interface{}{
					"Environment": "Production",
				},
			},
			expectedResult: false,
		},
		{
			name: "Multiple tags without monitoring tag",
			requestParameters: map[string]interface{}{
				"tags": map[string]interface{}{
					"Environment": "Production",
					"CostCenter":  "Engineering",
					"Team":        "Backend",
				},
			},
			expectedResult: false,
		},
		{
			name: "Multiple tags with monitoring tag",
			requestParameters: map[string]interface{}{
				"tags": map[string]interface{}{
					"Environment": "Production",
					"logzio:logs": "true",
					"CostCenter":  "Engineering",
				},
			},
			expectedResult: true,
		},
		{
			name:              "No tags field in requestParameters",
			requestParameters: map[string]interface{}{},
			expectedResult:    false,
		},
		{
			name: "Tags field is not a map",
			requestParameters: map[string]interface{}{
				"tags": "not-a-map",
			},
			expectedResult: false,
		},
		{
			name:              "Empty requestParameters",
			requestParameters: map[string]interface{}{},
			expectedResult:    false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := hasMonitoringTag(test.requestParameters)
			assert.Equal(t, test.expectedResult, result)
		})
	}
}

func TestHasMonitoringTagWithCustomConfig(t *testing.T) {
	ctx := setupHandlerTest()

	// Set custom monitoring tag configuration
	err := os.Setenv("MONITORING_TAG_KEY", "MyCustomTag")
	assert.Nil(t, err)
	err = os.Setenv("MONITORING_TAG_VALUE", "enabled")
	assert.Nil(t, err)
	// Enable tag events for this test
	err = os.Setenv("TAG_EVENTS_ENABLED", "true")
	assert.Nil(t, err)

	tests := []struct {
		name              string
		event             map[string]interface{}
		expectedOutputMsg string
		expectedError     bool
	}{
		{
			name: "TagResource with custom monitoring tag should be processed",
			event: map[string]interface{}{
				"detail": map[string]interface{}{
					"eventName": "TagResource",
					"requestParameters": map[string]interface{}{
						"resourceArn": "arn:aws:logs:us-east-1:123456789012:log-group:/aws/lambda/my-function",
						"tags": map[string]interface{}{
							"MyCustomTag": "enabled",
						},
					},
				},
			},
			expectedOutputMsg: "TagResource event handled successfully",
			expectedError:     false,
		},
		{
			name: "TagResource with custom tag in different case should be processed",
			event: map[string]interface{}{
				"detail": map[string]interface{}{
					"eventName": "TagResource",
					"requestParameters": map[string]interface{}{
						"resourceArn": "arn:aws:logs:us-east-1:123456789012:log-group:/aws/lambda/my-function",
						"tags": map[string]interface{}{
							"mycustomtag": "ENABLED",
						},
					},
				},
			},
			expectedOutputMsg: "TagResource event handled successfully",
			expectedError:     false,
		},
		{
			name: "TagResource with default tag should be skipped (custom config active)",
			event: map[string]interface{}{
				"detail": map[string]interface{}{
					"eventName": "TagResource",
					"requestParameters": map[string]interface{}{
						"resourceArn": "arn:aws:logs:us-east-1:123456789012:log-group:/aws/lambda/my-function",
						"tags": map[string]interface{}{
							"logzio:logs": "true",
						},
					},
				},
			},
			expectedOutputMsg: "TagResource event skipped - monitoring tag not present",
			expectedError:     false,
		},
		{
			name: "TagResource with wrong custom value should be skipped",
			event: map[string]interface{}{
				"detail": map[string]interface{}{
					"eventName": "TagResource",
					"requestParameters": map[string]interface{}{
						"resourceArn": "arn:aws:logs:us-east-1:123456789012:log-group:/aws/lambda/my-function",
						"tags": map[string]interface{}{
							"MyCustomTag": "disabled",
						},
					},
				},
			},
			expectedOutputMsg: "TagResource event skipped - monitoring tag not present",
			expectedError:     false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res, err := HandleRequest(ctx, test.event)

			if test.expectedError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
			assert.Equal(t, test.expectedOutputMsg, res)
		})
	}

	// Clean up custom environment variables
	os.Unsetenv("MONITORING_TAG_KEY")
	os.Unsetenv("MONITORING_TAG_VALUE")
	os.Unsetenv("TAG_EVENTS_ENABLED")
}

func TestTagEventsDisabledByDefault(t *testing.T) {
	ctx := setupHandlerTest()
	// Do not set TAG_EVENTS_ENABLED - should default to false

	tests := []struct {
		name              string
		event             map[string]interface{}
		expectedOutputMsg string
		expectedError     bool
	}{
		{
			name: "TagResource event should be skipped when TAG_EVENTS_ENABLED is not set",
			event: map[string]interface{}{
				"detail": map[string]interface{}{
					"eventName": "TagResource",
					"requestParameters": map[string]interface{}{
						"resourceArn": "arn:aws:logs:us-east-1:123456789012:log-group:/aws/lambda/my-function",
						"tags": map[string]interface{}{
							"logzio:logs": "true",
						},
					},
				},
			},
			expectedOutputMsg: "TagResource event skipped - feature disabled",
			expectedError:     false,
		},
		{
			name: "TagResource20170331v2 event should be skipped when TAG_EVENTS_ENABLED is not set",
			event: map[string]interface{}{
				"detail": map[string]interface{}{
					"eventName": "TagResource20170331v2",
					"requestParameters": map[string]interface{}{
						"resource": "arn:aws:lambda:us-east-1:123456789012:function:my-function",
						"tags": map[string]interface{}{
							"logzio:logs": "true",
						},
					},
				},
			},
			expectedOutputMsg: "TagResource20170331v2 event skipped - feature disabled",
			expectedError:     false,
		},
		{
			name: "CreateFunction20150331 event should be skipped when TAG_EVENTS_ENABLED is not set",
			event: map[string]interface{}{
				"detail": map[string]interface{}{
					"eventName": "CreateFunction20150331",
					"requestParameters": map[string]interface{}{
						"functionName": "my-function",
						"tags": map[string]interface{}{
							"logzio:logs": "true",
						},
					},
				},
			},
			expectedOutputMsg: "CreateFunction20150331 event skipped - feature disabled",
			expectedError:     false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res, err := HandleRequest(ctx, test.event)

			if test.expectedError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
			assert.Equal(t, test.expectedOutputMsg, res)
		})
	}
}

func TestTagEventsEnabled(t *testing.T) {
	ctx := setupHandlerTest()

	// Enable tag events
	err := os.Setenv("TAG_EVENTS_ENABLED", "true")
	assert.Nil(t, err)

	tests := []struct {
		name              string
		event             map[string]interface{}
		expectedOutputMsg string
		expectedError     bool
	}{
		{
			name: "TagResource event should be processed when TAG_EVENTS_ENABLED is true",
			event: map[string]interface{}{
				"detail": map[string]interface{}{
					"eventName": "TagResource",
					"requestParameters": map[string]interface{}{
						"resourceArn": "arn:aws:logs:us-east-1:123456789012:log-group:/aws/lambda/my-function",
						"tags": map[string]interface{}{
							"logzio:logs": "true",
						},
					},
				},
			},
			expectedOutputMsg: "TagResource event handled successfully",
			expectedError:     false,
		},
		{
			name: "TagResource20170331v2 event should be processed when TAG_EVENTS_ENABLED is true",
			event: map[string]interface{}{
				"detail": map[string]interface{}{
					"eventName": "TagResource20170331v2",
					"requestParameters": map[string]interface{}{
						"resource": "arn:aws:lambda:us-east-1:123456789012:function:my-function",
						"tags": map[string]interface{}{
							"logzio:logs": "true",
						},
					},
				},
			},
			expectedOutputMsg: "TagResource20170331v2 event handled successfully",
			expectedError:     false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res, err := HandleRequest(ctx, test.event)

			if test.expectedError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
			assert.Equal(t, test.expectedOutputMsg, res)
		})
	}

	// Clean up
	os.Unsetenv("TAG_EVENTS_ENABLED")
}

func TestTagResourceEventSkippedWithoutMonitoringTag(t *testing.T) {
	ctx := setupHandlerTest()

	// Enable tag events for this test
	err := os.Setenv("TAG_EVENTS_ENABLED", "true")
	assert.Nil(t, err)
	defer os.Unsetenv("TAG_EVENTS_ENABLED")

	tests := []struct {
		name              string
		event             map[string]interface{}
		expectedOutputMsg string
		expectedError     bool
	}{
		{
			name: "TagResource event without monitoring tag should be skipped",
			event: map[string]interface{}{
				"detail": map[string]interface{}{
					"eventName": "TagResource",
					"requestParameters": map[string]interface{}{
						"resourceArn": "arn:aws:logs:us-east-1:123456789012:log-group:/aws/lambda/my-function",
						"tags": map[string]interface{}{
							"Environment": "Production",
						},
					},
				},
			},
			expectedOutputMsg: "TagResource event skipped - monitoring tag not present",
			expectedError:     false,
		},
		{
			name: "TagResource event with monitoring tag: false should be skipped",
			event: map[string]interface{}{
				"detail": map[string]interface{}{
					"eventName": "TagResource",
					"requestParameters": map[string]interface{}{
						"resourceArn": "arn:aws:logs:us-east-1:123456789012:log-group:/aws/lambda/my-function",
						"tags": map[string]interface{}{
							"logzio:logs": "False",
						},
					},
				},
			},
			expectedOutputMsg: "TagResource event skipped - monitoring tag not present",
			expectedError:     false,
		},
		{
			name: "TagResource event without tags field should be skipped",
			event: map[string]interface{}{
				"detail": map[string]interface{}{
					"eventName": "TagResource",
					"requestParameters": map[string]interface{}{
						"resourceArn": "arn:aws:logs:us-east-1:123456789012:log-group:/aws/lambda/my-function",
					},
				},
			},
			expectedOutputMsg: "TagResource event skipped - monitoring tag not present",
			expectedError:     false,
		},
		{
			name: "TagResource20170331v2 event without monitoring tag should be skipped",
			event: map[string]interface{}{
				"detail": map[string]interface{}{
					"eventName": "TagResource20170331v2",
					"requestParameters": map[string]interface{}{
						"resource": "arn:aws:lambda:us-east-1:123456789012:function:my-payment-processor",
						"tags": map[string]interface{}{
							"CostCenter": "Engineering",
						},
					},
				},
			},
			expectedOutputMsg: "TagResource20170331v2 event skipped - monitoring tag not present",
			expectedError:     false,
		},
		{
			name: "TagResource20170331v2 event with monitoring tag: false should be skipped",
			event: map[string]interface{}{
				"detail": map[string]interface{}{
					"eventName": "TagResource20170331v2",
					"requestParameters": map[string]interface{}{
						"resource": "arn:aws:lambda:us-east-1:123456789012:function:my-payment-processor",
						"tags": map[string]interface{}{
							"logzio:logs": "false",
						},
					},
				},
			},
			expectedOutputMsg: "TagResource20170331v2 event skipped - monitoring tag not present",
			expectedError:     false,
		},
		{
			name: "TagResource event with monitoring tag: true should be processed",
			event: map[string]interface{}{
				"detail": map[string]interface{}{
					"eventName": "TagResource",
					"requestParameters": map[string]interface{}{
						"resourceArn": "arn:aws:logs:us-east-1:123456789012:log-group:/aws/lambda/my-function",
						"tags": map[string]interface{}{
							"logzio:logs": "true",
						},
					},
				},
			},
			expectedOutputMsg: "TagResource event handled successfully",
			expectedError:     false,
		},
		{
			name: "TagResource20170331v2 event with monitoring tag: true should be processed",
			event: map[string]interface{}{
				"detail": map[string]interface{}{
					"eventName": "TagResource20170331v2",
					"requestParameters": map[string]interface{}{
						"resource": "arn:aws:lambda:us-east-1:123456789012:function:my-payment-processor",
						"tags": map[string]interface{}{
							"logzio:logs": "true",
						},
					},
				},
			},
			expectedOutputMsg: "TagResource20170331v2 event handled successfully",
			expectedError:     false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res, err := HandleRequest(ctx, test.event)

			if test.expectedError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
			assert.Equal(t, test.expectedOutputMsg, res)
		})
	}
}

func TestCreateFunctionEventHandling(t *testing.T) {
	ctx := setupHandlerTest()

	err := os.Setenv(common.EnvAwsRegion, "us-east-1")
	assert.Nil(t, err)
	defer os.Unsetenv(common.EnvAwsRegion)

	tests := []struct {
		name              string
		tagEventsEnabled  bool
		event             map[string]interface{}
		expectedOutputMsg string
		expectedError     bool
	}{
		{
			name:             "CreateFunction20150331 event skipped when TAG_EVENTS_ENABLED is false",
			tagEventsEnabled: false,
			event: map[string]interface{}{
				"detail": map[string]interface{}{
					"eventName": "CreateFunction20150331",
					"requestParameters": map[string]interface{}{
						"functionName": "my-function",
						"tags": map[string]interface{}{
							"logzio:logs": "true",
						},
					},
				},
			},
			expectedOutputMsg: "CreateFunction20150331 event skipped - feature disabled",
			expectedError:     false,
		},
		{
			name:             "CreateFunction20150331 event skipped when monitoring tag is absent",
			tagEventsEnabled: true,
			event: map[string]interface{}{
				"detail": map[string]interface{}{
					"eventName": "CreateFunction20150331",
					"requestParameters": map[string]interface{}{
						"functionName": "my-function",
						"tags": map[string]interface{}{
							"Environment": "Production",
						},
					},
				},
			},
			expectedOutputMsg: "CreateFunction20150331 event skipped - monitoring tag not present",
			expectedError:     false,
		},
		{
			name:             "CreateFunction20150331 event skipped when monitoring tag value is false",
			tagEventsEnabled: true,
			event: map[string]interface{}{
				"detail": map[string]interface{}{
					"eventName": "CreateFunction20150331",
					"requestParameters": map[string]interface{}{
						"functionName": "my-function",
						"tags": map[string]interface{}{
							"logzio:logs": "false",
						},
					},
				},
			},
			expectedOutputMsg: "CreateFunction20150331 event skipped - monitoring tag not present",
			expectedError:     false,
		},
		{
			name:             "CreateFunction20150331 event skipped when no tags field present",
			tagEventsEnabled: true,
			event: map[string]interface{}{
				"detail": map[string]interface{}{
					"eventName": "CreateFunction20150331",
					"requestParameters": map[string]interface{}{
						"functionName": "my-function",
					},
				},
			},
			expectedOutputMsg: "CreateFunction20150331 event skipped - monitoring tag not present",
			expectedError:     false,
		},
		{
			name:             "CreateFunction20150331 event with missing functionName field",
			tagEventsEnabled: true,
			event: map[string]interface{}{
				"detail": map[string]interface{}{
					"eventName": "CreateFunction20150331",
					"requestParameters": map[string]interface{}{
						"tags": map[string]interface{}{
							"logzio:logs": "true",
						},
					},
				},
			},
			expectedOutputMsg: "",
			expectedError:     true,
		},
		{
			name:             "CreateFunction20150331 event handled successfully",
			tagEventsEnabled: true,
			event: map[string]interface{}{
				"detail": map[string]interface{}{
					"eventName": "CreateFunction20150331",
					"requestParameters": map[string]interface{}{
						"functionName": "my-function",
						"tags": map[string]interface{}{
							"logzio:logs": "true",
						},
					},
				},
			},
			expectedOutputMsg: "CreateFunction20150331 event handled successfully",
			expectedError:     false,
		},
		{
			name:             "CreateFunction20150331 event monitoring tag case insensitive",
			tagEventsEnabled: true,
			event: map[string]interface{}{
				"detail": map[string]interface{}{
					"eventName": "CreateFunction20150331",
					"requestParameters": map[string]interface{}{
						"functionName": "my-function",
						"tags": map[string]interface{}{
							"LOGZIO:LOGS": "TRUE",
						},
					},
				},
			},
			expectedOutputMsg: "CreateFunction20150331 event handled successfully",
			expectedError:     false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.tagEventsEnabled {
				err := os.Setenv("TAG_EVENTS_ENABLED", "true")
				assert.Nil(t, err)
			} else {
				os.Unsetenv("TAG_EVENTS_ENABLED")
			}

			res, err := HandleRequest(ctx, test.event)

			if test.expectedError {
				assert.NotNil(t, err)
				assert.Equal(t, test.expectedOutputMsg, res)
			} else {
				assert.Nil(t, err)
				assert.Equal(t, test.expectedOutputMsg, res)
			}
		})
	}

	os.Unsetenv("TAG_EVENTS_ENABLED")
}

func TestHandleTagResourceEvent(t *testing.T) {
	ctx := setupHandlerTest()
	
	// Initialize logger and config (normally done in HandleRequest)
	sugLog = logger.GetSugaredLogger()
	envConfig = NewConfig()

	tests := []struct {
		name             string
		taggedResource   string
		expectedError    bool
		expectedErrorMsg string
	}{
		{
			name:           "Valid CloudWatch Log Group ARN",
			taggedResource: "arn:aws:logs:us-east-1:123456789012:log-group:/aws/lambda/my-function",
			expectedError:  false,
		},
		{
			name:           "Valid Lambda Function ARN",
			taggedResource: "arn:aws:lambda:us-east-1:123456789012:function:my-payment-processor",
			expectedError:  false,
		},
		{
			name:             "Invalid ARN format - not an ARN",
			taggedResource:   "not-an-arn",
			expectedError:    true,
			expectedErrorMsg: "provided string is not AWS arn",
		},
		{
			name:             "Invalid ARN format - missing parts",
			taggedResource:   "arn:aws:logs",
			expectedError:    true,
			expectedErrorMsg: "provided string is not AWS arn",
		},
		{
			name:           "Valid Log Group ARN with colons in name",
			taggedResource: "arn:aws:logs:us-east-1:123456789012:log-group:/custom/path/with:colons:here",
			expectedError:  false,
		},
		{
			name:           "Valid Lambda ARN with alias",
			taggedResource: "arn:aws:lambda:us-east-1:123456789012:function:my-function:prod",
			expectedError:  false,
		},
		{
			name:             "Invalid resource type - S3",
			taggedResource:   "arn:aws:s3:::my-bucket/my-key",
			expectedError:    true,
			expectedErrorMsg: "unable to get name from arn.resource ",
		},
		{
			name:             "Invalid resource type - EC2",
			taggedResource:   "arn:aws:ec2:us-east-1:123456789012:instance/i-1234567890abcdef0",
			expectedError:    true,
			expectedErrorMsg: "unable to get name from arn.resource ",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res, err := handleTagResourceEvent(ctx, test.taggedResource)

			if test.expectedError {
				assert.NotNil(t, err)
				if test.expectedErrorMsg != "" {
					assert.Contains(t, err.Error(), test.expectedErrorMsg)
				}
			} else {
				assert.Nil(t, err)
				assert.Equal(t, "Tag Resource Event handled successfully.", res)
			}
		})
	}
}
