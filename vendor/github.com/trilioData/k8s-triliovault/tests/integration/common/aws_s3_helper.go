package common

import (
	"errors"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

func GetS3ClientService() (svc *s3.S3, err error) {

	// Check AWS Creds
	_, accessKeyPresent := os.LookupEnv(AWSAccessKeyID)
	_, secretKeyPresent := os.LookupEnv(AWSSecretAccessKey)
	if !accessKeyPresent || !secretKeyPresent {
		return nil, errors.New("")
	}

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("us-east-1")},
	)
	if err != nil {
		return nil, err
	}

	// Create S3 service client
	svc = s3.New(sess)

	return svc, nil
}

func CreateS3Bucket(svc *s3.S3, bucketName string) error {
	// Create the S3 Bucket
	_, err := svc.CreateBucket(&s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return err
	}

	err = svc.WaitUntilBucketExists(&s3.HeadBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return err
	}
	return nil
}

func DeleteS3Bucket(svc *s3.S3, bucketName string) error {
	_, err := svc.DeleteBucket(&s3.DeleteBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return err
	}

	err = svc.WaitUntilBucketNotExists(&s3.HeadBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return err
	}

	return err
}
