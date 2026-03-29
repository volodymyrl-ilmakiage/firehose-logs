package handler

const (
	envFunctionName              = "AWS_LAMBDA_FUNCTION_NAME" // reserved env
	envAccountId                 = "ACCOUNT_ID"
	envFirehoseArn               = "FIREHOSE_ARN"
	envAwsPartition              = "AWS_PARTITION"
	envPutSubscriptionFilterRole = "PUT_SF_ROLE"
	envStackName                 = "STACK_NAME"
	envFilterPattern             = "FILTER_PATTERN"
	envMonitoringTagKey          = "MONITORING_TAG_KEY"
	envMonitoringTagValue        = "MONITORING_TAG_VALUE"
	envTagEventsEnabled          = "TAG_EVENTS_ENABLED"

	logzioSecretKeyName       = "logzioCustomLogGroups"
	valuesSeparator           = ","
	emptyString               = ""
	lambdaPrefix              = "/aws/lambda/"
	subscriptionFilterName    = "logzio_firehose"
	maxRetries                = 10
	defaultMonitoringTagKey   = "logzio:logs"
	defaultMonitoringTagValue = "true"
)
