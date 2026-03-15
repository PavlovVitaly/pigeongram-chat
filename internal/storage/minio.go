package storage

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"pigeongram/config"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type FileInfo struct {
	Name        string    `json:"name"`
	Size        int64     `json:"size"`
	UploadedAt  time.Time `json:"uploadedAt"`
	ContentType string    `json:"contentType"`
	UploaderID  string    `json:"uploaderId"`
	ChatID      string    `json:"chatId"`
	Key         string    `json:"key"`
}

type MinIOClient struct {
	client         *minio.Client
	bucketName     string
	region         string
	uploadExpiry   time.Duration
	downloadExpiry time.Duration
	maxFileSize    int64
}

func NewMinIOClient(cfg *config.MinIOConfig) (*MinIOClient, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к MinIO: %w", err)
	}

	// Проверяем подключение
	ctx := context.Background()

	// Создаем bucket если не существует
	exists, err := client.BucketExists(ctx, cfg.BucketName)
	if err != nil {
		return nil, fmt.Errorf("ошибка проверки bucket: %w", err)
	}

	if !exists {
		err = client.MakeBucket(ctx, cfg.BucketName, minio.MakeBucketOptions{
			Region: cfg.Region,
		})
		if err != nil {
			return nil, fmt.Errorf("ошибка создания bucket: %w", err)
		}

		// Устанавливаем политику доступа (приватный)
		// Никто не имеет прямого доступа, только через presigned URLs
	}

	return &MinIOClient{
		client:         client,
		bucketName:     cfg.BucketName,
		region:         cfg.Region,
		uploadExpiry:   cfg.UploadExpiry,
		downloadExpiry: cfg.DownloadExpiry,
		maxFileSize:    cfg.MaxFileSize,
	}, nil
}

// GenerateUploadURL создает временную ссылку для загрузки
func (m *MinIOClient) GenerateUploadURL(ctx context.Context, chatID, userID, filename string) (string, map[string]string, error) {
	// Создаем уникальный ключ: chat-{chatID}/{userID}/{timestamp}-{filename}
	timestamp := time.Now().Unix()
	safeFilename := filepath.Base(filename) // защита от path traversal
	objectKey := fmt.Sprintf("chat-%s/%s/%d-%s", chatID, userID, timestamp, safeFilename)

	policy := minio.NewPostPolicy()
	policy.SetBucket(m.bucketName)
	policy.SetKey(objectKey)
	policy.SetExpires(time.Now().Add(m.uploadExpiry))

	// Ограничения на файл
	policy.SetContentLengthRange(1, m.maxFileSize)

	// Разрешенные типы файлов (опционально)
	// policy.SetContentType("image/*")

	url, formData, err := m.client.PresignedPostPolicy(ctx, policy)
	if err != nil {
		return "", nil, fmt.Errorf("ошибка создания presigned URL: %w", err)
	}

	return url.String(), formData, nil
}

// GenerateDownloadURL создает временную ссылку для скачивания
func (m *MinIOClient) GenerateDownloadURL(ctx context.Context, chatID, objectKey string) (string, error) {
	// Временно отключаем проверку для отладки
	// expectedPrefix := fmt.Sprintf("chat-%s/", chatID)
	// if !strings.HasPrefix(objectKey, expectedPrefix) {
	//     return "", fmt.Errorf("доступ запрещен: файл не принадлежит чату")
	// }

	log.Printf("🔍 [MinIO] Генерация ссылки на скачивание: key=%s", objectKey)

	reqParams := make(url.Values)
	reqParams.Set("response-content-disposition", "attachment")

	presignedURL, err := m.client.PresignedGetObject(ctx, m.bucketName, objectKey, m.downloadExpiry, reqParams)
	if err != nil {
		log.Printf("❌ [MinIO] Ошибка генерации ссылки: %v", err)
		return "", fmt.Errorf("ошибка создания presigned URL: %w", err)
	}

	log.Printf("✅ [MinIO] Ссылка сгенерирована: %s", presignedURL.String())
	return presignedURL.String(), nil
}

// ListFiles возвращает список файлов в чате
func (m *MinIOClient) ListFiles(ctx context.Context, chatID string) ([]FileInfo, error) {
	prefix := fmt.Sprintf("chat-%s/", chatID)

	objects := m.client.ListObjects(ctx, m.bucketName, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})

	var files []FileInfo
	for obj := range objects {
		if obj.Err != nil {
			return nil, obj.Err
		}

		// Парсим userID и имя файла из пути
		parts := strings.Split(obj.Key, "/")

		var userID, filename string
		if len(parts) >= 3 {
			userID = parts[1]
			// убираем timestamp из имени
			filenameParts := strings.SplitN(parts[2], "-", 2)
			if len(filenameParts) == 2 {
				filename = filenameParts[1]
			} else {
				filename = parts[2]
			}
		}

		files = append(files, FileInfo{
			Name:        filename,
			Size:        obj.Size,
			UploadedAt:  obj.LastModified,
			ContentType: obj.ContentType,
			UploaderID:  userID,
			ChatID:      chatID,
			Key:         obj.Key,
		})
	}

	// ✅ Всегда возвращаем массив (даже пустой)
	return files, nil
}

// DeleteFile удаляет файл
func (m *MinIOClient) DeleteFile(ctx context.Context, chatID, objectKey string) error {
	// Проверяем права доступа
	expectedPrefix := fmt.Sprintf("chat-%s/", chatID)
	if !strings.HasPrefix(objectKey, expectedPrefix) {
		return fmt.Errorf("доступ запрещен: файл не принадлежит чату")
	}

	return m.client.RemoveObject(ctx, m.bucketName, objectKey, minio.RemoveObjectOptions{})
}

// GetFileInfo получает информацию о файле
func (m *MinIOClient) GetFileInfo(ctx context.Context, chatID, objectKey string) (*FileInfo, error) {
	// Временно отключаем строгую проверку для отладки
	// expectedPrefix := fmt.Sprintf("chat-%s/", chatID)
	// if !strings.HasPrefix(objectKey, expectedPrefix) {
	//     return nil, fmt.Errorf("доступ запрещен: файл не принадлежит чату (ожидалось %s, получено %s)",
	//         expectedPrefix, objectKey)
	// }

	log.Printf("🔍 [MinIO] Получение информации о файле: bucket=%s, key=%s", m.bucketName, objectKey)

	obj, err := m.client.StatObject(ctx, m.bucketName, objectKey, minio.StatObjectOptions{})
	if err != nil {
		log.Printf("❌ [MinIO] Ошибка StatObject: %v", err)
		return nil, err
	}

	// Парсим информацию из ключа
	parts := strings.Split(objectKey, "/")
	userID := "unknown"
	filename := objectKey

	if len(parts) >= 3 {
		userID = parts[1]
		filename = parts[len(parts)-1] // берем последнюю часть как имя файла
	}

	// Удаляем временную метку из имени если есть
	if idx := strings.Index(filename, "-"); idx > 0 && len(filename) > idx+1 {
		// Проверяем, что первая часть похожа на timestamp (все цифры)
		potentialTimestamp := filename[:idx]
		if strings.Trim(potentialTimestamp, "0123456789") == "" {
			filename = filename[idx+1:]
		}
	}

	log.Printf("✅ [MinIO] Информация получена: имя=%s, размер=%d, пользователь=%s",
		filename, obj.Size, userID)

	return &FileInfo{
		Name:        filename,
		Size:        obj.Size,
		UploadedAt:  obj.LastModified,
		ContentType: obj.ContentType,
		UploaderID:  userID,
		ChatID:      chatID,
		Key:         objectKey,
	}, nil
}

// GetBucketName возвращает имя bucket
func (m *MinIOClient) GetBucketName() string {
	return m.bucketName
}
