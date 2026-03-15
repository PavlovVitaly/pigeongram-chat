package config

import (
	"time"
)

type MinIOConfig struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
	BucketName      string
	Region          string
	UploadExpiry    time.Duration
	DownloadExpiry  time.Duration
	MaxFileSize     int64
}

func NewMinIOConfig() *MinIOConfig {
	return &MinIOConfig{
		Endpoint:        GetEnv("MINIO_ENDPOINT", "localhost:9000"),
		AccessKeyID:     GetEnv("MINIO_ACCESS_KEY", "minioadmin"),
		SecretAccessKey: GetEnv("MINIO_SECRET_KEY", "minioadmin"),
		UseSSL:          GetEnvAsBool("MINIO_USE_SSL", false),
		BucketName:      GetEnv("MINIO_BUCKET", "pigeongram-files"),
		Region:          GetEnv("MINIO_REGION", "us-east-1"),
		UploadExpiry:    GetEnvAsDuration("MINIO_UPLOAD_EXPIRY", 15*time.Minute),
		DownloadExpiry:  GetEnvAsDuration("MINIO_DOWNLOAD_EXPIRY", 24*time.Hour),
		MaxFileSize:     GetEnvAsInt64("MINIO_MAX_FILE_SIZE", 100<<20), // 100 MB
	}
}
