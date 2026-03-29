package handler

import (
	"context"
	"fmt"

	awsArn "github.com/aws/aws-sdk-go/aws/arn"

	"strings"

	"github.com/logzio/firehose-logs/common"
	"github.com/logzio/firehose-logs/logger"
	"go.uber.org/zap"
)

var sugLog *zap.SugaredLogger
var envConfig *Config

func HandleRequest(ctx context.Context, event map[string]interface{}) (string, error) {
	sugLog = logger.GetSugaredLogger()

	envConfig = NewConfig()
	if envConfig == nil {
		return "Lambda finished with error", fmt.Errorf("error while validating required environment variables")
	}

	sugLog.Info("Starting handling event...")
	sugLog.Debug("Handling event: ", event)

	detail, ok := event["detail"].(map[string]interface{})
	if !ok {
		sugLog.Error("`detail` is not of type map[string]interface{} or missing from the event.")
	}

	eventName, ok := detail["eventName"].(string)
	if !ok {
		sugLog.Error("`eventName` is not of type string or missing from the event.")
	}

	requestParameters, ok := detail["requestParameters"].(map[string]interface{})
	if !ok {
		sugLog.Error("`requestParameters` is not of type map[string]interface{} or missing from the event.")
	}

	switch eventName {
	case "CreateLogGroup":
		sugLog.Debug("Detected EventBridge CreateLogGroup event")

		logGroup, ok := requestParameters["logGroupName"].(string)
		if !ok {
			sugLog.Error("`logGroupName` is not of type string or missing from EventBridge event")
			return "", fmt.Errorf("`logGroupName` is not of type string or missing from EventBridge event")
		}
		handleNewLogGroupEvent(ctx, logGroup)

	case "PutSecretValue":
		sugLog.Debug("Detected EventBridge PutSecretValue event")

		secretId, ok := requestParameters["secretId"].(string)
		if !ok {
			sugLog.Error("`secretId` is not of type string or missing from EventBridge event")
			return "", fmt.Errorf("`secretId` is not of type string or missing from EventBridge event")
		}
		err := handleSecretChangedEvent(ctx, secretId)
		if err != nil {
			return "", err
		}

	case "TagResource":
		sugLog.Debug("Detected EventBridge TagResource event")

		if !envConfig.tagEventsEnabled {
			sugLog.Debug("Skipping TagResource event - TAG_EVENTS_ENABLED is not set to true")
			return "TagResource event skipped - feature disabled", nil
		}

		taggedResource, ok := requestParameters["resourceArn"].(string)
		if !ok {
			sugLog.Errorf("`resourceArn` is not of type string or missing from EventBridge event")
			return "", fmt.Errorf("`resourceArn` is not of type string or missing from EventBridge event")
		}

		// Check if the monitoring tag is present
		if !hasMonitoringTag(requestParameters) {
			sugLog.Debugf("Skipping TagResource event - monitoring tag %s: %s not present", envConfig.monitoringTagKey, envConfig.monitoringTagValue)
			return "TagResource event skipped - monitoring tag not present", nil
		}

		_, err := handleTagResourceEvent(ctx, taggedResource)
		if err != nil {
			return "", err
		}

	case "TagResource20170331v2":
		sugLog.Debug("Detected EventBridge TagResource20170331v2 event")

		if !envConfig.tagEventsEnabled {
			sugLog.Debug("Skipping TagResource20170331v2 event - TAG_EVENTS_ENABLED is not set to true")
			return "TagResource20170331v2 event skipped - feature disabled", nil
		}

		taggedResource, ok := requestParameters["resource"].(string)
		if !ok {
			sugLog.Errorf("`resource` is not of type string or missing from EventBridge event.")
			return "", fmt.Errorf("`resource` is not of type string or missing from EventBridge event")
		}

		if !hasMonitoringTag(requestParameters) {
			sugLog.Debugf("Skipping TagResource20170331v2 event - monitoring tag %s: %s not present", envConfig.monitoringTagKey, envConfig.monitoringTagValue)
			return "TagResource20170331v2 event skipped - monitoring tag not present", nil
		}

		_, err := handleTagResourceEvent(ctx, taggedResource)

		if err != nil {
			return "", err
		}

	case "CreateFunction20150331":
		sugLog.Debug("Detected EventBridge CreateFunction20150331 event")

		if !envConfig.tagEventsEnabled {
			sugLog.Debug("Skipping CreateFunction20150331 event - TAG_EVENTS_ENABLED is not set to true")
			return "CreateFunction20150331 event skipped - feature disabled", nil
		}

		functionName, ok := requestParameters["functionName"].(string)
		if !ok {
			sugLog.Errorf("`functionName` is not of type string or missing from EventBridge event.")
			return "", fmt.Errorf("`functionName` is not of type string or missing from EventBridge event")
		}
		taggedResource := fmt.Sprintf("arn:aws:lambda:%s:%s:function:%s", envConfig.region, envConfig.accountId, functionName)

		if !hasMonitoringTag(requestParameters) {
			sugLog.Debugf("Skipping CreateFunction20150331 event - monitoring tag %s: %s not present", envConfig.monitoringTagKey, envConfig.monitoringTagValue)
			return "CreateFunction20150331 event skipped - monitoring tag not present", nil
		}

		_, err := handleTagResourceEvent(ctx, taggedResource)

		if err != nil {
			return "", err
		}

	case "SubscriptionFilterEvent":
		sugLog.Debug("Detected SubscriptionFilterEvent event")

		var reqParams common.RequestParameters
		reqParams, err := common.ConvertToRequestParameters(requestParameters)
		if err != nil {
			sugLog.Error("Error converting request parameters: ", err.Error())
			return "", err
		}

		actionType := reqParams.Action
		switch actionType {
		case common.AddSF:
			sugLog.Debug("Detected Add Subscription Filter event")
			handleCreateEvent(ctx, reqParams)
		case common.UpdateSF:
			sugLog.Debug("Detected Update Subscription Filter event")
			handleUpdateEvent(ctx, reqParams)
		case common.DeleteSF:
			sugLog.Debug("Detected Delete Subscription Filter event")
			return handleDeleteEvent(ctx, reqParams)
		default:
			sugLog.Debug("Detected unsupported Subscription Filter event")
			return "", fmt.Errorf("unsupported Subscription Filter event")
		}

	default:
		sugLog.Debug("Detected unsupported event")
		return "", fmt.Errorf("unsupported event")
	}

	return fmt.Sprintf("%s event handled successfully", eventName), nil
}

func handleNewLogGroupEvent(ctx context.Context, newLogGroup string) {
	// Prevent a situation where we put subscription filter on the trigger function
	if newLogGroup == envConfig.thisFunctionLogGroup {
		return
	}

	// Check if subscription filter already exists to avoid redundant API calls
	cwClient, err := getCloudWatchLogsClient()
	if err != nil {
		sugLog.Error("Failed to get cloudwatch logs client: ", err)
		return
	}

	hasFilter, err := cwClient.hasSubscriptionFilter(newLogGroup)
	if err != nil {
		sugLog.Debug("Error checking if subscription filter exists for ", newLogGroup, ": ", err)
		// Continue anyway.  If the check fails (network issue, temporary API error, etc.), we don't want to completely give up
	} else if hasFilter {
		sugLog.Debug("Subscription filter already exists for log group ", newLogGroup, ", skipping")
		return
	}

	// Check if the log group is of a monitored service
	currMonitoredServices := getServices()
	var added []string
	if currMonitoredServices != nil {
		serviceToPrefix := getServicesMap()

		for _, service := range currMonitoredServices {
			if prefix, ok := serviceToPrefix[service]; ok {
				if strings.Contains(newLogGroup, prefix) {
					var err error
					added, err = cwClient.addSubscriptionFilter([]string{newLogGroup})
					if err != nil {
						sugLog.Errorf("Failed to add subscription filter to log group %s: %v", newLogGroup, err)
					}
					if len(added) > 0 {
						sugLog.Info("Added subscription filter to log group: ", newLogGroup)
						return
					}
				}
			}
		}
	}

	// Check if the log group is of a monitored custom prefix
	currCustomGroupsPrefixes := getCustomGroupsPrefixes()
	if len(currCustomGroupsPrefixes) > 0 {
		for _, prefix := range currCustomGroupsPrefixes {
			if strings.Contains(newLogGroup, prefix) {
				var err error
				added, err = cwClient.addSubscriptionFilter([]string{newLogGroup})
				if err != nil {
					sugLog.Errorf("Failed to add subscription filter to log group %s: %v", newLogGroup, err)
				}
				if len(added) > 0 {
					sugLog.Info("Added subscription filter to log group: ", newLogGroup)
					return
				}
			}
		}
	}
}

func handleSecretChangedEvent(ctx context.Context, secretId string) error {
	secretName := envConfig.customGroupsValue

	// make sure that the secret which changed is the relevant secret
	if strings.Contains(secretId, secretName) {
		err := updateSecretCustomLogGroups(ctx, secretId)
		if err != nil {
			sugLog.Error("Error while updating secret custom log groups: ", err.Error())
			return err
		}
	} else {
		sugLog.Debug("The EventBridge event secretId is not the secret that has custom log groups in it. Skipping it.")
	}
	return nil
}

func handleCreateEvent(ctx context.Context, event common.RequestParameters) {
	cwClient, err := getCloudWatchLogsClient()
	if err != nil {
		sugLog.Error("Failed to get cloudwatch logs client")
		return
	}

	servicesToMonitor := convertStrToArr(event.NewServices)
	logGroupsToMonitor := getServicesLogGroups(servicesToMonitor, cwClient)

	customLogGroupsToMonitor, err := getCustomLogGroups(event.NewIsSecret, event.NewCustom)
	if err != nil {
		sugLog.Error("Error while getting custom log groups: ", err.Error())
	}
	logGroupsToMonitor = append(logGroupsToMonitor, customLogGroupsToMonitor...)

	added, _ := cwClient.addSubscriptionFilter(logGroupsToMonitor)

	sugLog.Info("Added subscription filters for the following log groups: ", added)
}

func handleUpdateEvent(ctx context.Context, event common.RequestParameters) {
	cwClient, err := getCloudWatchLogsClient()
	if err != nil {
		sugLog.Error("Failed to get cloudwatch logs client")
	}

	oldServices := convertStrToArr(event.OldServices)
	newServices := convertStrToArr(event.NewServices)

	oldCustomGroups, err := getCustomLogGroups(event.OldIsSecret, event.OldCustom)
	if err != nil {
		sugLog.Error("Error while getting old custom log groups: ", err.Error())
	}
	newCustomGroups, err := getCustomLogGroups(event.NewIsSecret, event.NewCustom)
	if err != nil {
		sugLog.Error("Error while getting new custom log groups: ", err.Error())
	}

	servicesToAdd, servicesToRemove := findDifferences(oldServices, newServices)
	customGroupsToAdd, customGroupsToRemove := findDifferences(oldCustomGroups, newCustomGroups)

	err = cwClient.updateSubscriptionFilters(servicesToAdd, servicesToRemove, customGroupsToAdd, customGroupsToRemove)
	if err != nil {
		sugLog.Error("Error while updating subscription filters: ", err.Error())
	}
}

func handleDeleteEvent(ctx context.Context, event common.RequestParameters) (string, error) {
	cwClient, err := getCloudWatchLogsClient()
	if err != nil {
		sugLog.Error("Failed to get cloudwatch logs client")
		return "", err
	}

	servicesToUnMonitor := convertStrToArr(event.NewServices)
	logGroupsToUnMonitor := getServicesLogGroups(servicesToUnMonitor, cwClient)

	customLogGroupsToUnMonitor, err := getCustomLogGroups(event.NewIsSecret, event.NewCustom)
	if err != nil {
		sugLog.Error("Error while getting custom log groups: ", err.Error())
		return "", err
	}

	logGroupsToUnMonitor = append(logGroupsToUnMonitor, customLogGroupsToUnMonitor...)
	deleted, err := cwClient.removeSubscriptionFilter(logGroupsToUnMonitor)
	if err != nil {
		sugLog.Error("Error while removing subscription filters: ", err.Error())
		return "", err
	}

	sugLog.Info("Deleted subscription filters for the following log groups: ", deleted)
	return "Event handled successfully", nil
}

// hasMonitoringTag checks if the configured monitoring tag is present in the request parameters
func hasMonitoringTag(requestParameters map[string]interface{}) bool {
	tags, ok := requestParameters["tags"].(map[string]interface{})
	if !ok {
		sugLog.Debug("No tags found in requestParameters or tags is not a map")
		return false
	}

	expectedKey := envConfig.monitoringTagKey
	expectedValue := envConfig.monitoringTagValue

	// Check for monitoring tag (case-insensitive) with expected value (case-insensitive)
	for key, value := range tags {
		if strings.EqualFold(key, expectedKey) {
			if valueStr, ok := value.(string); ok && strings.EqualFold(valueStr, expectedValue) {
				sugLog.Debugf("Found monitoring tag %s: %s", expectedKey, expectedValue)
				return true
			}
		}
	}

	sugLog.Debugf("Monitoring tag %s: %s not found", expectedKey, expectedValue)
	return false
}

func handleTagResourceEvent(ctx context.Context, taggedResource string) (string, error) {
	if !awsArn.IsARN(taggedResource) {
		return "", fmt.Errorf("provided string is not AWS arn")
	}

	resourceType, err := awsArn.Parse(taggedResource)
	if err != nil {
		return "", fmt.Errorf("unable to parse aws arn")
	}

	parts := strings.SplitN(resourceType.Resource, ":", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("unable to get name from arn.resource ")
	}

	switch resourceType.Service {
	case "lambda":
		handleNewLogGroupEvent(ctx, fmt.Sprintf("/aws/lambda/%s", parts[1]))
	case "logs":
		handleNewLogGroupEvent(ctx, parts[1])
	}

	return "Tag Resource Event handled successfully.", nil
}
