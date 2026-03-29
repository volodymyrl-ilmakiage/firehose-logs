package handler

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs/cloudwatchlogsiface"
	lp "github.com/logzio/firehose-logs/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"os"
	"sort"
	"testing"
)

type MockCloudWatchLogsClient struct {
	mock.Mock
	cloudwatchlogsiface.CloudWatchLogsAPI
}

func (m *MockCloudWatchLogsClient) PutSubscriptionFilter(input *cloudwatchlogs.PutSubscriptionFilterInput) (*cloudwatchlogs.PutSubscriptionFilterOutput, error) {
	if *input.LogGroupName == "errorGroup" {
		return nil, fmt.Errorf("an error occurred")
	}

	args := m.Called(input)
	return args.Get(0).(*cloudwatchlogs.PutSubscriptionFilterOutput), args.Error(1)
}

func (m *MockCloudWatchLogsClient) DeleteSubscriptionFilter(input *cloudwatchlogs.DeleteSubscriptionFilterInput) (*cloudwatchlogs.DeleteSubscriptionFilterOutput, error) {
	if *input.LogGroupName == "errorGroup" {
		return nil, fmt.Errorf("an error occurred")
	}

	args := m.Called(input)
	return args.Get(0).(*cloudwatchlogs.DeleteSubscriptionFilterOutput), args.Error(1)
}

func (m *MockCloudWatchLogsClient) DescribeLogGroups(input *cloudwatchlogs.DescribeLogGroupsInput) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
	switch *input.LogGroupNamePrefix {
	case "/aws/apigateway/":
		return &cloudwatchlogs.DescribeLogGroupsOutput{
			LogGroups: []*cloudwatchlogs.LogGroup{
				{
					LogGroupName: aws.String("/aws/apigateway/g1"),
				}},
		}, nil
	case "/aws/lambda/":
		return &cloudwatchlogs.DescribeLogGroupsOutput{
			LogGroups: []*cloudwatchlogs.LogGroup{{
				LogGroupName: aws.String("/aws/lambda/g1"),
			},
				{
					LogGroupName: aws.String("/aws/lambda/g2"),
				}},
		}, nil
	case "/aws/codebuild/":
		return &cloudwatchlogs.DescribeLogGroupsOutput{
			LogGroups: []*cloudwatchlogs.LogGroup{},
		}, nil
	case "/log/group1/":
		return &cloudwatchlogs.DescribeLogGroupsOutput{
			LogGroups: []*cloudwatchlogs.LogGroup{{
				LogGroupName: aws.String("/log/group1/a"),
			},
				{
					LogGroupName: aws.String("/log/group1/b"),
				}},
		}, nil
	case "/log/group2/":
		return &cloudwatchlogs.DescribeLogGroupsOutput{
			LogGroups: []*cloudwatchlogs.LogGroup{
				{
					LogGroupName: aws.String("/log/group2/a"),
				}},
		}, nil
	case "/aws/error/test/":
		return &cloudwatchlogs.DescribeLogGroupsOutput{
			LogGroups: []*cloudwatchlogs.LogGroup{},
		}, fmt.Errorf("an error occurred")
	default:
		return &cloudwatchlogs.DescribeLogGroupsOutput{
			LogGroups: []*cloudwatchlogs.LogGroup{{
				LogGroupName: aws.String("/random/log/group"),
			}},
		}, nil
	}
}

func (m *MockCloudWatchLogsClient) DescribeSubscriptionFilters(input *cloudwatchlogs.DescribeSubscriptionFiltersInput) (*cloudwatchlogs.DescribeSubscriptionFiltersOutput, error) {
	switch *input.LogGroupName {
	case "/aws/lambda/with-filter":
		// Filter with matching destination ARN (test-arn from setupSFTest)
		return &cloudwatchlogs.DescribeSubscriptionFiltersOutput{
			SubscriptionFilters: []*cloudwatchlogs.SubscriptionFilter{
				{
					FilterName:     aws.String("test-stack_logzio_firehose"),
					DestinationArn: aws.String("test-arn"),  // Matches envConfig.destinationArn
				},
			},
		}, nil
	case "/aws/lambda/without-filter":
		return &cloudwatchlogs.DescribeSubscriptionFiltersOutput{
			SubscriptionFilters: []*cloudwatchlogs.SubscriptionFilter{},
		}, nil
	case "/aws/lambda/different-filter":
		// Filter with different destination ARN
		return &cloudwatchlogs.DescribeSubscriptionFiltersOutput{
			SubscriptionFilters: []*cloudwatchlogs.SubscriptionFilter{
				{
					FilterName:     aws.String("some-other-filter"),
					DestinationArn: aws.String("arn:aws:firehose:us-east-1:123456789012:deliverystream/other"),
				},
			},
		}, nil
	case "/aws/lambda/error-group":
		// Return proper AWS error type
		return nil, awserr.New("ResourceNotFoundException", "log group does not exist", nil)
	default:
		return &cloudwatchlogs.DescribeSubscriptionFiltersOutput{
			SubscriptionFilters: []*cloudwatchlogs.SubscriptionFilter{},
		}, nil
	}
}

func setupSFTest() {
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

	err = os.Setenv(envStackName, "test-stack")
	if err != nil {
		return
	}

	/* Setup config */
	envConfig = NewConfig()

	/* Setup logger */
	sugLog = lp.GetSugaredLogger()
}

func TestAddSubscriptionFilter(t *testing.T) {
	setupSFTest()

	tests := []struct {
		name          string
		logGroups     []string
		expectedAdded []string
		errorExpected bool
	}{
		{
			name:          "All successful",
			logGroups:     []string{"group1", "group2"},
			expectedAdded: []string{"group1", "group2"},
			errorExpected: false,
		},
		{
			name:          "Error on one group",
			logGroups:     []string{"group1", "errorGroup"},
			expectedAdded: []string{"group1"},
			errorExpected: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockClient := new(MockCloudWatchLogsClient)
			mockClient.On("PutSubscriptionFilter", mock.Anything).Return(&cloudwatchlogs.PutSubscriptionFilterOutput{}, nil).Times(len(test.logGroups))
			mockClient.On("PutSubscriptionFilter", mock.MatchedBy(func(input *cloudwatchlogs.PutSubscriptionFilterInput) bool {
				return *input.LogGroupName == "errorGroup"
			})).Return(nil, fmt.Errorf("an error occurred"))

			cwClient := &CloudWatchLogsClient{Client: mockClient}
			added, err := cwClient.addSubscriptionFilter(test.logGroups)
			sort.Strings(added)

			assert.Equal(t, test.expectedAdded, added, "Expected log groups to be added %v but got %v", test.expectedAdded, added)

			if test.errorExpected {
				assert.NotNil(t, err, "Expected an error but got nil")
			} else {
				assert.Nil(t, err, "Expected error to be nil but got %v", err)
			}
		})
	}
}

func TestUpdateSubscriptionFilters(t *testing.T) {
	setupSFTest()

	tests := []struct {
		name                 string
		servicesToAdd        []string
		servicesToRemove     []string
		customGroupsToAdd    []string
		customGroupsToRemove []string
		errorExpected        bool
	}{
		{
			name:                 "new to add and delete",
			servicesToAdd:        []string{"apigateway", "lambda"},
			servicesToRemove:     []string{"rds"},
			customGroupsToAdd:    []string{"group1"},
			customGroupsToRemove: []string{"group2"},
			errorExpected:        false,
		},
		{
			name:                 "no new to add",
			servicesToAdd:        []string{},
			servicesToRemove:     []string{"lambda"},
			customGroupsToAdd:    []string{},
			customGroupsToRemove: []string{"group2"},
			errorExpected:        false,
		},
		{
			name:                 "no new to delete",
			servicesToAdd:        []string{"apigateway", "rds"},
			servicesToRemove:     []string{},
			customGroupsToAdd:    []string{"group1"},
			customGroupsToRemove: []string{},
			errorExpected:        false,
		},
		{
			name:                 "error in add",
			servicesToAdd:        []string{},
			servicesToRemove:     []string{},
			customGroupsToAdd:    []string{"errorGroup"},
			customGroupsToRemove: []string{},
			errorExpected:        true,
		},
		{
			name:                 "error in delete",
			servicesToAdd:        []string{},
			servicesToRemove:     []string{},
			customGroupsToAdd:    []string{},
			customGroupsToRemove: []string{"errorGroup"},
			errorExpected:        true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockClient := new(MockCloudWatchLogsClient)
			mockClient.On("PutSubscriptionFilter", mock.Anything).Return(&cloudwatchlogs.PutSubscriptionFilterOutput{}, nil)
			mockClient.On("PutSubscriptionFilter", mock.MatchedBy(func(input *cloudwatchlogs.PutSubscriptionFilterInput) bool {
				return *input.LogGroupName == "errorGroup"
			})).Return(nil, fmt.Errorf("an error occurred"))

			mockClient.On("DeleteSubscriptionFilter", mock.Anything).Return(&cloudwatchlogs.DeleteSubscriptionFilterOutput{}, nil)
			mockClient.On("DeleteSubscriptionFilter", mock.MatchedBy(func(input *cloudwatchlogs.DeleteSubscriptionFilterInput) bool {
				return *input.LogGroupName == "errorGroup"
			})).Return(nil, fmt.Errorf("an error occurred"))

			cwClient := &CloudWatchLogsClient{Client: mockClient}
			err := cwClient.updateSubscriptionFilters(test.servicesToAdd, test.servicesToRemove, test.customGroupsToAdd, test.customGroupsToRemove)

			if test.errorExpected {
				assert.NotNil(t, err, "Expected an error but got nil")
			} else {
				assert.Nil(t, err, "Expected error to be nil but got %v", err)
			}
		})
	}
}

func TestDeleteSubscriptionFilters(t *testing.T) {
	setupSFTest()

	tests := []struct {
		name           string
		logGroups      []string
		expectedRemove []string
		errorExpected  bool
	}{
		{
			name:           "All successful",
			logGroups:      []string{"group1", "group2"},
			expectedRemove: []string{"group1", "group2"},
			errorExpected:  false,
		},
		{
			name:           "error on one group",
			logGroups:      []string{"group1", "errorGroup"},
			expectedRemove: []string{"group1"},
			errorExpected:  true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockClient := new(MockCloudWatchLogsClient)
			mockClient.On("DeleteSubscriptionFilter", mock.Anything).Return(&cloudwatchlogs.DeleteSubscriptionFilterOutput{}, nil).Times(len(test.logGroups))
			mockClient.On("DeleteSubscriptionFilter", mock.MatchedBy(func(input *cloudwatchlogs.DeleteSubscriptionFilterInput) bool {
				return *input.LogGroupName == "errorGroup"
			})).Return(nil, fmt.Errorf("an error occurred"))

			cwClient := &CloudWatchLogsClient{Client: mockClient}
			removed, err := cwClient.removeSubscriptionFilter(test.logGroups)
			sort.Strings(removed)

			assert.Equal(t, test.expectedRemove, removed, "Expected log groups to be removed %v but got %v", test.expectedRemove, removed)

			if test.errorExpected {
				assert.NotNil(t, err, "Expected an error but got nil")
			} else {
				assert.Nil(t, err, "Expected error to be nil but got %v", err)
			}
		})
	}
}

func TestGetLogGroupsWithPrefix(t *testing.T) {
	cwClient, _ := setupLGTest()

	tests := []struct {
		name           string
		prefix         string
		expectedGroups []string
		expectedError  bool
	}{
		{
			name:           "some prefix",
			prefix:         "/aws/apigateway/",
			expectedGroups: []string{"/aws/apigateway/g1"},
			expectedError:  false,
		},
		{
			name:           "no log groups",
			prefix:         "/aws/codebuild/",
			expectedGroups: []string{},
			expectedError:  false,
		},
		{
			name:           "don't return this function log group",
			prefix:         "/aws/lambda/",
			expectedGroups: []string{"/aws/lambda/g1"},
			expectedError:  false,
		},
		{
			name:           "failed to get log groups",
			prefix:         "/aws/error/test/",
			expectedGroups: nil,
			expectedError:  true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := cwClient.getLogGroupsWithPrefix(test.prefix)
			sort.Strings(result)
			assert.Equal(t, test.expectedGroups, result)

			if test.expectedError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestHasSubscriptionFilter(t *testing.T) {
	setupSFTest()
	sugLog = lp.GetSugaredLogger()
	envConfig = NewConfig()

	mockClient := new(MockCloudWatchLogsClient)
	cwClient := &CloudWatchLogsClient{Client: mockClient}

	tests := []struct {
		name           string
		logGroupName   string
		expectedExists bool
		expectedError  bool
	}{
		{
			name:           "Log group with matching destination ARN",
			logGroupName:   "/aws/lambda/with-filter",
			expectedExists: true,
			expectedError:  false,
		},
		{
			name:           "Log group without subscription filter",
			logGroupName:   "/aws/lambda/without-filter",
			expectedExists: false,
			expectedError:  false,
		},
		{
			name:           "Log group with different destination ARN",
			logGroupName:   "/aws/lambda/different-filter",
			expectedExists: false,
			expectedError:  false,
		},
		{
			name:           "Log group does not exist (ResourceNotFoundException)",
			logGroupName:   "/aws/lambda/error-group",
			expectedExists: false,
			expectedError:  false, // ResourceNotFoundException should return false, not error
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			exists, err := cwClient.hasSubscriptionFilter(test.logGroupName)

			assert.Equal(t, test.expectedExists, exists, "Expected exists to be %v but got %v", test.expectedExists, exists)

			if test.expectedError {
				assert.NotNil(t, err, "Expected an error but got nil")
			} else {
				assert.Nil(t, err, "Expected error to be nil but got %v", err)
			}
		})
	}
}
