package main

// required packages
import (
	"bytes"
	"context"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/networkfirewall"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// constants
const (
	S3BucketName = "placeholder" // example: "network-firewall-backups-00000"
	S3Folder     = "placeholder" // example: "suricata_backups"
	Region       = "placeholder" // example: "us-west-1"
)

// rule group data structure
type RuleGroup struct {
	Name string
	Arn  string
}

// list all rule groups in region
func ListRuleGroups(ctx context.Context, client *networkfirewall.Client) ([]RuleGroup, error) {
	log.Println("Listing rule groups...")
	var ruleGroups []RuleGroup
	paginator := networkfirewall.NewListRuleGroupsPaginator(client, &networkfirewall.ListRuleGroupsInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			log.Printf("Error fetching rule groups page: %v", err)
			return nil, err
		}
		for _, rg := range page.RuleGroups {
			ruleGroups = append(ruleGroups, RuleGroup{
				Name: aws.ToString(rg.Name),
				Arn:  aws.ToString(rg.Arn),
			})
		}
	}
	log.Printf("Found %d rule groups.", len(ruleGroups))
	return ruleGroups, nil
}

// fetch all rules in each rule group
func FetchSuricataRules(ctx context.Context, client *networkfirewall.Client, ruleGroupArn string) (string, error) {
	log.Printf("Fetching Suricata rules for Rule Group ARN: %s", ruleGroupArn)
	resp, err := client.DescribeRuleGroup(ctx, &networkfirewall.DescribeRuleGroupInput{
		RuleGroupArn: aws.String(ruleGroupArn),
		Type:         "STATEFUL",
	})
	if err != nil {
		log.Printf("Error fetching Suricata rules for ARN %s: %v", ruleGroupArn, err)
		return "", err
	}
	if rulesSource := resp.RuleGroup.RulesSource; rulesSource != nil && rulesSource.RulesString != nil {
		log.Printf("Successfully fetched rules for ARN: %s", ruleGroupArn)
		return aws.ToString(rulesSource.RulesString), nil
	}
	log.Printf("No rules found for ARN: %s", ruleGroupArn)
	return "", nil
}

// upload rules by rule group to s3
func UploadToS3(ctx context.Context, client *s3.Client, content, fileName, bucketName, folder string) error {
	key := fmt.Sprintf("%s/%s", folder, fileName)
	log.Printf("Uploading to S3 bucket %s, key: %s", bucketName, key)
	body := bytes.NewReader([]byte(content))
	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
		Body:   body,
	})
	if err != nil {
		log.Printf("Error uploading file %s to S3: %v", fileName, err)
		return err
	}
	log.Printf("Successfully uploaded file %s to S3.", fileName)
	return nil
}

// main Lambda funtion start
func LambdaHandler(ctx context.Context) (string, error) {
	log.Println("Loading AWS configuration...")
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(Region))
	if err != nil {
		log.Printf("Error loading AWS configuration: %v", err)
		return "", err
	}

	log.Println("Initializing AWS clients...")
	networkFirewallClient := networkfirewall.NewFromConfig(cfg)
	s3Client := s3.NewFromConfig(cfg)

	log.Println("Starting backup process...")
	ruleGroups, err := ListRuleGroups(ctx, networkFirewallClient)
	if err != nil {
		log.Printf("Error listing rule groups: %v", err)
		return "", err
	}

	for _, rg := range ruleGroups {
		log.Printf("Processing rule group: %s", rg.Name)
		rules, err := FetchSuricataRules(ctx, networkFirewallClient, rg.Arn)
		if err != nil || rules == "" {
			log.Printf("Skipping rule group %s due to error or no rules.", rg.Name)
			continue
		}

		timestamp := time.Now().Format("2006-01-02_15-04-05")
		fileName := fmt.Sprintf("%s_%s.txt", rg.Name, timestamp)
		err = UploadToS3(ctx, s3Client, rules, fileName, S3BucketName, S3Folder)
		if err != nil {
			log.Printf("Failed to upload rules for %s: %v", rg.Name, err)
		}
	}

	log.Println("Backup process completed successfully.")
	return "Backup completed successfully.", nil
}

func main() {
	log.Println("Starting Lambda function...")
	lambda.Start(LambdaHandler)
}
