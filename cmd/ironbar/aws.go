package main

import (
	"context"
	"errors"
	"fmt"

	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/aws/smithy-go"
)

func isTaskActive(ctx context.Context, cfg aws.Config, ecsClusterArn, taskArn string) (bool, error) {
	logger := slog.With("arn", taskArn, "cluster_arn", ecsClusterArn)
	logger.Debug("checking if task is active")

	svc := ecs.NewFromConfig(cfg)
	in := &ecs.DescribeTasksInput{
		Cluster: aws.String(ecsClusterArn),
		Tasks: []string{
			*aws.String(taskArn),
		},
	}

	out, err := svc.DescribeTasks(ctx, in)
	if err != nil {
		return true, fmt.Errorf("describe tasks: %w", err)
	}
	for _, ta := range out.Tasks {
		if ta.TaskArn != nil && *ta.TaskArn == taskArn {
			if ta.LastStatus != nil {
				logger.Debug("task found", "status", *ta.LastStatus)
				if *ta.LastStatus == "DEACTIVATING" ||
					*ta.LastStatus == "STOPPING" ||
					*ta.LastStatus == "DEPROVISIONING" ||
					*ta.LastStatus == "STOPPED" ||
					*ta.LastStatus == "DELETED" {
					return false, nil
				}
				return true, nil
			}

			logger.Warn("task found, but cannot read status")
			return true, nil
		}
	}
	logger.Debug("task not found")
	return false, nil
}

func isTaskDefinitionActive(ctx context.Context, cfg aws.Config, arn string) (bool, error) {
	logger := slog.With("arn", arn)
	logger.Debug("checking if task definition is active")

	svc := ecs.NewFromConfig(cfg)
	in := &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(arn),
	}

	out, err := svc.DescribeTaskDefinition(ctx, in)
	if err != nil {
		return true, fmt.Errorf("describe task definition: %w", err)
	}

	if out.TaskDefinition != nil && out.TaskDefinition.TaskDefinitionArn != nil && *out.TaskDefinition.TaskDefinitionArn == arn {
		if out.TaskDefinition.Status == ecstypes.TaskDefinitionStatusActive {
			logger.Debug("task definition active")
			return true, nil
		}
		logger.Debug("task definition not active")
		return false, nil
	}

	logger.Debug("task definition not found")
	return false, nil
}

func isSnsSubscriptionActive(ctx context.Context, cfg aws.Config, arn string) (bool, error) {
	logger := slog.With("arn", arn)
	logger.Debug("checking if subscription is active")

	svc := sns.NewFromConfig(cfg)
	in := &sns.GetSubscriptionAttributesInput{
		SubscriptionArn: aws.String(arn),
	}

	out, err := svc.GetSubscriptionAttributes(ctx, in)
	if err != nil {
		var apiError smithy.APIError
		if errors.As(err, &apiError) {
			switch apiError.(type) {
			case *snstypes.NotFoundException:
				return false, nil
			}
		}
		return true, fmt.Errorf("get subscription attributes: %w", err)
	}

	if out == nil || out.Attributes == nil {
		logger.Debug("no attributes found")
		return false, nil
	}

	if out.Attributes["TopicArn"] != "" {
		logger.Debug("topic found")
		return true, nil
	}

	logger.Debug("no topic found")
	return false, nil
}

func isSqsQueueActive(ctx context.Context, cfg aws.Config, url string) (bool, error) {
	logger := slog.With("url", url)
	logger.Debug("checking if queue is active")

	svc := sqs.NewFromConfig(cfg)
	in := &sqs.GetQueueAttributesInput{
		QueueUrl: aws.String(url),
		AttributeNames: []sqstypes.QueueAttributeName{
			sqstypes.QueueAttributeNameQueueArn,
		},
	}

	out, err := svc.GetQueueAttributes(ctx, in)
	if err != nil {
		var apiError smithy.APIError
		if errors.As(err, &apiError) {
			switch apiError.(type) {
			case *snstypes.NotFoundException:
				return false, nil
			}
		}
		return true, fmt.Errorf("get subscription attributes: %w", err)
	}

	if out == nil || out.Attributes == nil {
		logger.Debug("no attributes found")
		return false, nil
	}

	if out.Attributes["QueueArn"] != "" {
		logger.Debug("queue arn found")
		return true, nil
	}

	logger.Debug("no queue found")
	return false, nil
}

func stopEcsTask(ctx context.Context, cfg aws.Config, ecsClusterArn, taskArn string) error {
	svc := ecs.NewFromConfig(cfg)

	in := &ecs.StopTaskInput{
		Cluster: aws.String(ecsClusterArn),
		Task:    aws.String(taskArn),
	}

	if _, err := svc.StopTask(ctx, in); err != nil {
		return fmt.Errorf("stop task: %w", err)
	}
	return nil
}

func deregisterEcsTaskDefinition(ctx context.Context, cfg aws.Config, arn string) error {
	in := &ecs.DeregisterTaskDefinitionInput{
		TaskDefinition: aws.String(arn),
	}

	svc := ecs.NewFromConfig(cfg)
	_, err := svc.DeregisterTaskDefinition(ctx, in)
	if err != nil {
		return fmt.Errorf("deregister task definition: %w", err)
	}
	return nil
}

func unsubscribeSqsQueue(ctx context.Context, cfg aws.Config, arn string) error {
	snssvc := sns.NewFromConfig(cfg)

	in := &sns.UnsubscribeInput{
		SubscriptionArn: aws.String(arn),
	}

	_, err := snssvc.Unsubscribe(ctx, in)
	if err != nil {
		return fmt.Errorf("unsubscribe from request topic: %w", err)
	}

	return nil
}

func deleteSqsQueue(ctx context.Context, cfg aws.Config, queueURL string) error {
	svc := sqs.NewFromConfig(cfg)

	in := &sqs.DeleteQueueInput{
		QueueUrl: aws.String(queueURL),
	}

	_, err := svc.DeleteQueue(ctx, in)
	if err != nil {
		return fmt.Errorf("delete queue: %w", err)
	}
	return nil
}

func isEc2InstanceActive(ctx context.Context, cfg aws.Config, instanceID string) (bool, error) {
	logger := slog.With("instance_id", instanceID)
	logger.Debug("checking if ec2 instance is active")

	svc := ec2.NewFromConfig(cfg)
	in := &ec2.DescribeInstancesInput{
		InstanceIds: []string{
			instanceID,
		},
	}
	out, err := svc.DescribeInstances(ctx, in)
	if err != nil {
		return true, fmt.Errorf("describe ec2 instances: %w", err)
	}

	switch len(out.Reservations) {
	case 0:
		logger.Debug("no reservations found")
		return false, nil
	case 1:
		logger.Debug("reservation still active")
		return true, nil
	default:
		logger.Warn(fmt.Sprintf("unexpected number of instance reservations found: %d", len(out.Reservations)))
		return true, nil
	}
}
