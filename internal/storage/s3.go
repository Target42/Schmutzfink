package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

type S3 struct {
	client *s3.Client
	bucket string
	prefix string
}

func NewS3(ctx context.Context, opt Options) (*S3, error) {
	bucket := strings.TrimSpace(opt.Bucket)
	if bucket == "" {
		return nil, errors.New("S3_BUCKET fehlt")
	}
	region := strings.TrimSpace(opt.Region)
	if region == "" {
		region = "us-east-1"
	}
	access := strings.TrimSpace(opt.AccessKey)
	secret := strings.TrimSpace(opt.SecretKey)
	if access == "" || secret == "" {
		return nil, errors.New("S3_ACCESS_KEY oder S3_SECRET_KEY fehlt")
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(access, secret, "")),
	)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if ep := strings.TrimSpace(opt.Endpoint); ep != "" {
			o.BaseEndpoint = aws.String(ep)
			o.UsePathStyle = true
		}
		// MinIO rejects the default CRC checksums of recent AWS SDK versions.
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	})

	store := &S3{
		client: client,
		bucket: bucket,
		prefix: strings.Trim(strings.ReplaceAll(strings.TrimSpace(opt.Prefix), "\\", "/"), "/"),
	}
	if err := store.ensureBucket(ctx); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *S3) key(id string) (string, error) {
	k, err := objectKey(id)
	if err != nil {
		return "", err
	}
	if s.prefix != "" {
		return path.Join(s.prefix, k), nil
	}
	return k, nil
}

func (s *S3) ensureBucket(ctx context.Context) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(s.bucket)})
	if err == nil {
		return nil
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NotFound", "NoSuchBucket", "404":
			_, createErr := s.client.CreateBucket(ctx, &s3.CreateBucketInput{
				Bucket: aws.String(s.bucket),
			})
			if createErr == nil {
				return nil
			}
			var exists *types.BucketAlreadyOwnedByYou
			var taken *types.BucketAlreadyExists
			if errors.As(createErr, &exists) || errors.As(createErr, &taken) {
				return nil
			}
			return fmt.Errorf("s3 bucket %s: %w", s.bucket, createErr)
		}
	}
	return fmt.Errorf("s3 bucket %s: %w", s.bucket, err)
}

func (s *S3) Put(ctx context.Context, id string, r io.Reader) error {
	key, err := s.key(id)
	if err != nil {
		return err
	}
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   r,
	})
	return err
}

func (s *S3) Open(ctx context.Context, id string) (io.ReadCloser, error) {
	key, err := s.key(id)
	if err != nil {
		return nil, err
	}
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	return out.Body, nil
}

func (s *S3) Delete(ctx context.Context, id string) error {
	key, err := s.key(id)
	if err != nil {
		return err
	}
	_, err = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	return err
}
