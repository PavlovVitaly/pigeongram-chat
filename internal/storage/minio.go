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
	publicEndpoint string
}

func NewMinIOClient(cfg *config.MinIOConfig) (*MinIOClient, error) {
	// Для локальной разработки используем insecure
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к MinIO: %w", err)
	}

	ctx := context.Background()

	// Проверяем подключение
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
		fmt.Printf("✅ Bucket '%s' создан\n", cfg.BucketName)
	}

	return &MinIOClient{
		client:         client,
		bucketName:     cfg.BucketName,
		region:         cfg.Region,
		uploadExpiry:   cfg.UploadExpiry,
		downloadExpiry: cfg.DownloadExpiry,
		maxFileSize:    cfg.MaxFileSize,
		publicEndpoint: cfg.PublicEndpoint,
	}, nil
}

// GetBucketName возвращает имя bucket
func (m *MinIOClient) GetBucketName() string {
	return m.bucketName
}

// ListBuckets возвращает список всех bucket'ов
func (m *MinIOClient) ListBuckets(ctx context.Context) ([]string, error) {
	buckets, err := m.client.ListBuckets(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]string, len(buckets))
	for i, b := range buckets {
		result[i] = b.Name
	}
	return result, nil
}

// GenerateUploadURL создает временную ссылку для загрузки
func (m *MinIOClient) GenerateUploadURL(ctx context.Context, chatID, userID, filename string) (string, map[string]string, error) {
	timestamp := time.Now().Unix()
	safeFilename := filepath.Base(filename)

	// Очистка имени файла от спецсимволов
	safeFilename = strings.ReplaceAll(safeFilename, "'", "_")
	safeFilename = strings.ReplaceAll(safeFilename, "\"", "_")
	safeFilename = strings.ReplaceAll(safeFilename, "`", "_")
	safeFilename = strings.ReplaceAll(safeFilename, " ", "_")
	safeFilename = strings.ReplaceAll(safeFilename, "?", "_")
	safeFilename = strings.ReplaceAll(safeFilename, "*", "_")
	safeFilename = strings.ReplaceAll(safeFilename, ":", "_")
	safeFilename = strings.ReplaceAll(safeFilename, "<", "_")
	safeFilename = strings.ReplaceAll(safeFilename, ">", "_")
	safeFilename = strings.ReplaceAll(safeFilename, "|", "_")

	objectKey := fmt.Sprintf("chat-%s/%s/%d-%s", chatID, userID, timestamp, safeFilename)

	log.Printf("📦 [MinIO] Ключ объекта: %s", objectKey)

	policy := minio.NewPostPolicy()
	policy.SetBucket(m.bucketName)
	policy.SetKey(objectKey)
	policy.SetExpires(time.Now().Add(m.uploadExpiry))
	policy.SetContentLengthRange(1, m.maxFileSize)

	// Получаем внутренний URL от MinIO клиента
	internalURL, formData, err := m.client.PresignedPostPolicy(ctx, policy)
	if err != nil {
		return "", nil, fmt.Errorf("ошибка создания presigned URL: %w", err)
	}

	// Нормализуем публичный эндпоинт
	publicURL := normalizePublicEndpoint(m.publicEndpoint)

	if publicURL != nil {
		// Заменяем хост на публичный
		internalURL.Host = publicURL.Host
		// Сохраняем схему (http/https) из публичного эндпоинта
		internalURL.Scheme = publicURL.Scheme
	}

	return internalURL.String(), formData, nil
}

// GenerateDownloadURL - создает ссылку для скачивания
func (m *MinIOClient) GenerateDownloadURL(ctx context.Context, chatID, objectKey string) (string, error) {
	log.Printf("🔍 [MinIO] Генерация ссылки на скачивание: bucket=%s, key=%s",
		m.bucketName, objectKey)

	// Проверяем, что файл принадлежит чату
	expectedPrefix := fmt.Sprintf("chat-%s/", chatID)
	if !strings.HasPrefix(objectKey, expectedPrefix) {
		return "", fmt.Errorf("доступ запрещен: файл не принадлежит чату %s", chatID)
	}

	// Проверяем существование файла
	obj, err := m.client.StatObject(ctx, m.bucketName, objectKey, minio.StatObjectOptions{})
	if err != nil {
		log.Printf("❌ [MinIO] Файл не найден: %v", err)
		return "", fmt.Errorf("file not found: %w", err)
	}

	log.Printf("✅ [MinIO] Файл найден: %s, размер: %d", obj.Key, obj.Size)

	reqParams := make(url.Values)
	reqParams.Set("response-content-disposition", "attachment")

	presignedURL, err := m.client.PresignedGetObject(ctx, m.bucketName, objectKey, m.downloadExpiry, reqParams)
	if err != nil {
		log.Printf("❌ [MinIO] Ошибка генерации ссылки: %v", err)
		return "", fmt.Errorf("ошибка создания presigned URL: %w", err)
	}

	// Нормализуем публичный эндпоинт
	publicURL := normalizePublicEndpoint(m.publicEndpoint)

	if publicURL != nil {
		// Заменяем хост на публичный
		presignedURL.Host = publicURL.Host
		// Сохраняем схему (http/https) из публичного эндпоинта
		presignedURL.Scheme = publicURL.Scheme
	}

	urlStr := presignedURL.String()
	log.Printf("✅ [MinIO] Ссылка сгенерирована: %s", urlStr)

	return urlStr, nil
}

// normalizePublicEndpoint нормализует публичный эндпоинт
// Поддерживает форматы:
//   - "92.255.108.89:9000"
//   - "http://92.255.108.89:9000"
//   - "https://92.255.108.89:9000"
//   - "http://92.255.108.89:9000/"
//   - "92.255.108.89:9000/"
func normalizePublicEndpoint(endpoint string) *url.URL {
	if endpoint == "" {
		return nil
	}

	// Добавляем протокол если его нет
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		endpoint = "http://" + endpoint
	}

	// Убираем лишние слеши в конце
	endpoint = strings.TrimSuffix(endpoint, "/")

	// Парсим URL
	parsed, err := url.Parse(endpoint)
	if err != nil {
		log.Printf("⚠️ [MinIO] Ошибка парсинга publicEndpoint '%s': %v", endpoint, err)
		return nil
	}

	return parsed
}

// replaceHostInURL заменяет хост в URL на публичный
func replaceHostInURL(originalURL *url.URL, publicHost string) string {
	if publicHost == "" {
		return originalURL.String()
	}

	// Создаем копию URL
	result := *originalURL
	result.Host = publicHost

	return result.String()
}

// DeleteFile - удаляет файл (ТОЛЬКО ДЛЯ ВЛАДЕЛЬЦА)
func (m *MinIOClient) DeleteFile(ctx context.Context, chatID, objectKey, username string) error {
	log.Printf("🔍 [MinIO] Попытка удаления: bucket=%s, key=%s, user=%s",
		m.bucketName, objectKey, username)

	// Проверяем, что файл принадлежит чату
	expectedPrefix := fmt.Sprintf("chat-%s/", chatID)
	if !strings.HasPrefix(objectKey, expectedPrefix) {
		return fmt.Errorf("доступ запрещен: файл не принадлежит чату %s", chatID)
	}

	// Проверяем существование файла перед удалением
	obj, err := m.client.StatObject(ctx, m.bucketName, objectKey, minio.StatObjectOptions{})
	if err != nil {
		log.Printf("❌ [MinIO] Файл не найден: %v", err)
		return fmt.Errorf("file not found: %w", err)
	}

	log.Printf("✅ [MinIO] Файл найден: %s", obj.Key)

	// Определяем владельца файла из ключа
	parts := strings.Split(objectKey, "/")
	if len(parts) < 2 {
		return fmt.Errorf("некорректный формат ключа файла")
	}

	fileOwner := parts[1]
	log.Printf("👤 [MinIO] Владелец файла: %s, запросил: %s", fileOwner, username)

	if fileOwner != username {
		return fmt.Errorf("доступ запрещен: только владелец может удалить файл")
	}

	err = m.client.RemoveObject(ctx, m.bucketName, objectKey, minio.RemoveObjectOptions{})
	if err != nil {
		log.Printf("❌ [MinIO] Ошибка удаления: %v", err)
		return fmt.Errorf("ошибка удаления файла: %w", err)
	}

	log.Printf("✅ [MinIO] Файл успешно удален: %s", objectKey)
	return nil
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

		parts := strings.Split(obj.Key, "/")

		var userID, filename string

		if len(parts) >= 2 {
			userID = parts[1]

			if len(parts) >= 3 {
				filenamePart := parts[len(parts)-1]
				if idx := strings.Index(filenamePart, "-"); idx > 0 {
					potentialTimestamp := filenamePart[:idx]
					if strings.Trim(potentialTimestamp, "0123456789") == "" {
						filename = filenamePart[idx+1:]
					} else {
						filename = filenamePart
					}
				} else {
					filename = filenamePart
				}
			} else {
				filename = obj.Key
			}
		} else {
			userID = "unknown"
			filename = obj.Key
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

	log.Printf("📁 [MinIO] Найдено %d файлов в чате %s", len(files), chatID)
	return files, nil
}

// GetFileInfo - улучшаем парсинг имени файла
func (m *MinIOClient) GetFileInfo(ctx context.Context, chatID, objectKey string) (*FileInfo, error) {
	log.Printf("🔍 [MinIO] Получение информации о файле: %s", objectKey)

	obj, err := m.client.StatObject(ctx, m.bucketName, objectKey, minio.StatObjectOptions{})
	if err != nil {
		log.Printf("❌ [MinIO] Ошибка StatObject: %v", err)
		return nil, err
	}

	parts := strings.Split(objectKey, "/")
	userID := "unknown"
	filename := objectKey

	if len(parts) >= 2 {
		userID = parts[1]

		if len(parts) >= 3 {
			filenamePart := parts[len(parts)-1]
			// Убираем timestamp из имени
			if idx := strings.Index(filenamePart, "-"); idx > 0 {
				potentialTimestamp := filenamePart[:idx]
				if strings.Trim(potentialTimestamp, "0123456789") == "" {
					filename = filenamePart[idx+1:]
				} else {
					filename = filenamePart
				}
			} else {
				filename = filenamePart
			}

			// Восстанавливаем пробелы и специальные символы для отображения
			filename = strings.ReplaceAll(filename, "_", " ")
		}
	}

	log.Printf("✅ [MinIO] Информация получена: имя=%s, пользователь=%s", filename, userID)

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

// GetObject возвращает объект из MinIO для прямого скачивания
func (m *MinIOClient) GetObject(ctx context.Context, objectKey string) (*minio.Object, error) {
	log.Printf("📥 [MinIO] Получение объекта: %s", objectKey)

	obj, err := m.client.GetObject(ctx, m.bucketName, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("ошибка получения объекта: %w", err)
	}

	return obj, nil
}
