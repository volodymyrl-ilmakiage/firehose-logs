package handler

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs/cloudwatchlogsiface"
	"github.com/hashicorp/go-multierror"
	"github.com/logzio/firehose-logs/common"
)

type CloudWatchLogsClient struct {
	Client cloudwatchlogsiface.CloudWatchLogsAPI
	Mutex  sync.Mutex
}

func getCloudWatchLogsClient() (*CloudWatchLogsClient, error) {
	sess, err := common.GetSession()
	if err != nil {
		return nil, err
	}
	return &CloudWatchLogsClient{Client: cloudwatchlogs.New(sess)}, nil
}

func (cwLogsClient *CloudWatchLogsClient) addSubscriptionFilter(logGroups []string) ([]string, error) {
	if cwLogsClient == nil {
		return nil, fmt.Errorf("CloudWatch Logs client is nil")
	}

	destinationArn := envConfig.destinationArn
	roleArn := envConfig.roleArn
	filterPattern := envConfig.filterPattern
	filterName := envConfig.filterName
	added := make([]string, 0, len(logGroups))
	if filterPattern != "" {
		sugLog.Debugf("Applying filter pattern '%s' to log groups %s", filterPattern, logGroups)
	}
	var result *multierror.Error

	var wg sync.WaitGroup

	for _, logGroup := range logGroups {
		// Prevent a situation where we put subscription filter on the trigger function
		if logGroup == envConfig.thisFunctionLogGroup {
			continue
		}

		wg.Add(1)
		go func(logGroup string) {
			defer wg.Done()

			retries := 0
			for {
				filterInput := &cloudwatchlogs.PutSubscriptionFilterInput{
					DestinationArn: &destinationArn,
					FilterName:     &filterName,
					LogGroupName:   &logGroup,
					FilterPattern:  &filterPattern,
					RoleArn:        &roleArn,
				}
				_, err := cwLogsClient.Client.PutSubscriptionFilter(filterInput)

				// retry mechanism
				if err != nil {
					var awsErr awserr.Error
					ok := errors.As(err, &awsErr)
					if ok && awsErr.Code() == "ThrottlingException" && retries < maxRetries {
						time.Sleep(time.Second * time.Duration(retries*retries))
						retries++
						continue
					} else if ok && awsErr.Code() == "LimitExceededException" {
						sugLog.Warnf("Limit exceeded while trying to add subscription filter for %s: %v", logGroup, err.Error())
						return
					} else if ok && awsErr.Code() == "ResourceNotFoundException" {
						sugLog.Errorf("Log group %s does not exist - subscription filter cannot be added", logGroup)
						result = multierror.Append(result, err)
						return
					} else {
						sugLog.Errorf("Error while trying to add subscription filter for %s: %v", logGroup, err.Error())
						result = multierror.Append(result, err)
						return
					}
				}
				cwLogsClient.Mutex.Lock()
				added = append(added, logGroup)
				cwLogsClient.Mutex.Unlock()
				return
			}
		}(logGroup)
	}
	wg.Wait()

	return added, result.ErrorOrNil()
}

func (cwLogsClient *CloudWatchLogsClient) updateSubscriptionFilters(servicesToAdd, servicesToRemove, customGroupsToAdd, customGroupsToRemove []string) error {
	var result *multierror.Error

	logGroupsToMonitor := getServicesLogGroups(servicesToAdd, cwLogsClient)
	logGroupsToMonitor = append(logGroupsToMonitor, customGroupsToAdd...)

	if len(logGroupsToMonitor) > 0 {
		added, err := cwLogsClient.addSubscriptionFilter(logGroupsToMonitor)
		if err != nil {
			result = multierror.Append(result, err)
		}
		sugLog.Info("Added subscription filters for the following log groups: ", added)
	} else {
		sugLog.Debug("No new log groups to monitor")
	}

	logGroupsToUnMonitor := getServicesLogGroups(servicesToRemove, cwLogsClient)
	logGroupsToUnMonitor = append(logGroupsToUnMonitor, customGroupsToRemove...)

	if len(logGroupsToUnMonitor) > 0 {
		deleted, err := cwLogsClient.removeSubscriptionFilter(logGroupsToUnMonitor)
		if err != nil {
			result = multierror.Append(result, err)
		}
		sugLog.Info("Deleted subscription filters for the following log groups: ", deleted)
	} else {
		sugLog.Debug("No log groups to stop monitoring")
	}

	return result.ErrorOrNil()
}

func (cwLogsClient *CloudWatchLogsClient) removeSubscriptionFilter(logGroups []string) ([]string, error) {
	if cwLogsClient == nil {
		return nil, fmt.Errorf("CloudWatch Logs client is nil")
	}

	filterName := envConfig.filterName
	deleted := make([]string, 0, len(logGroups))
	var result *multierror.Error

	var wg sync.WaitGroup

	for _, logGroup := range logGroups {
		wg.Add(1)
		go func(logGroup string) {
			defer wg.Done()

			retries := 0
			for {
				_, err := cwLogsClient.Client.DeleteSubscriptionFilter(&cloudwatchlogs.DeleteSubscriptionFilterInput{
					FilterName:   &filterName,
					LogGroupName: &logGroup,
				})

				// retry mechanism
				if err != nil {
					var awsErr awserr.Error
					ok := errors.As(err, &awsErr)
					if ok && awsErr.Code() == "ThrottlingException" && retries < maxRetries {
						time.Sleep(time.Second * time.Duration(retries*retries))
						retries++
						continue
					} else {
						sugLog.Errorf("Error while trying to delete subscription filter for %s: %v", logGroup, err.Error())
						result = multierror.Append(result, err)
						return
					}
				}
				cwLogsClient.Mutex.Lock()
				deleted = append(deleted, logGroup)
				cwLogsClient.Mutex.Unlock()
				return
			}
		}(logGroup)
	}
	wg.Wait()

	return deleted, result.ErrorOrNil()
}

// getLogGroupsWithPrefix returns a list of log groups with the given prefix from cw client
func (cwLogsClient *CloudWatchLogsClient) getLogGroupsWithPrefix(prefix string) ([]string, error) {
	var nextToken *string
	logGroups := make([]string, 0)
	for {
		describeOutput, err := cwLogsClient.Client.DescribeLogGroups(&cloudwatchlogs.DescribeLogGroupsInput{
			LogGroupNamePrefix: &prefix,
			NextToken:          nextToken,
		})

		if err != nil {
			return nil, err
		}
		if describeOutput != nil {
			nextToken = describeOutput.NextToken
			for _, logGroup := range describeOutput.LogGroups {
				// Prevent a situation where we put subscription filter on the trigger and shipper function
				if *logGroup.LogGroupName != envConfig.thisFunctionLogGroup {
					logGroups = append(logGroups, *logGroup.LogGroupName)
				}
			}
		}

		if nextToken == nil {
			break
		}
	}

	return logGroups, nil
}

func (cwLogsClient *CloudWatchLogsClient) hasSubscriptionFilter(logGroupName string) (bool, error) {
	if cwLogsClient == nil {
		return false, fmt.Errorf("CloudWatch Logs client is nil")
	}

	destinationArn := envConfig.destinationArn

	output, err := cwLogsClient.Client.DescribeSubscriptionFilters(&cloudwatchlogs.DescribeSubscriptionFiltersInput{
		LogGroupName: &logGroupName,
	})

	if err != nil {
		var awsErr awserr.Error
		ok := errors.As(err, &awsErr)
		if ok && awsErr.Code() == "ResourceNotFoundException" {
			return false, nil
		}
		return false, err
	}

	for _, filter := range output.SubscriptionFilters {
		if filter.DestinationArn != nil && *filter.DestinationArn == destinationArn {
			return true, nil
		}
	}

	return false, nil
}
