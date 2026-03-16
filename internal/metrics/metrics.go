package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Активные подключения
	ActiveConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "pigeongram_connections_active",
		Help: "Текущее количество активных WebSocket соединений",
	})

	// Всего сообщений
	MessagesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "pigeongram_messages_total",
		Help: "Общее количество отправленных сообщений",
	})

	// Сообщения в секунду (для rate)
	MessagesPerSecond = promauto.NewCounter(prometheus.CounterOpts{
		Name: "pigeongram_messages_per_second",
		Help: "Количество сообщений в секунду",
	})

	// Загруженные файлы
	FilesUploadedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "pigeongram_files_uploaded_total",
		Help: "Общее количество загруженных файлов",
	})

	// Размер файлов
	FilesSizeBytes = promauto.NewCounter(prometheus.CounterOpts{
		Name: "pigeongram_files_size_bytes",
		Help: "Общий размер загруженных файлов в байтах",
	})

	// HTTP запросы по эндпоинтам
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pigeongram_http_requests_total",
			Help: "Общее количество HTTP запросов",
		},
		[]string{"method", "endpoint", "status"},
	)

	// Время ответа HTTP
	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "pigeongram_http_request_duration_seconds",
			Help:    "Длительность HTTP запросов в секундах",
			Buckets: []float64{0.1, 0.3, 0.5, 1, 2, 5, 10},
		},
		[]string{"method", "endpoint"},
	)

	// Ошибки WebSocket
	WebSocketErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pigeongram_websocket_errors_total",
			Help: "Количество ошибок WebSocket",
		},
		[]string{"type"},
	)

	// Размер очереди сообщений
	WebSocketQueueSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pigeongram_websocket_queue_size",
			Help: "Размер очереди отправки WebSocket",
		},
		[]string{"username"},
	)

	// Операции с БД
	DatabaseOperations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pigeongram_database_operations_total",
			Help: "Количество операций с базой данных",
		},
		[]string{"operation", "table"},
	)

	// Время ответа БД
	DatabaseDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "pigeongram_database_duration_seconds",
			Help:    "Длительность запросов к БД",
			Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1},
		},
		[]string{"operation"},
	)

	// Кэш операции
	CacheOperations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pigeongram_cache_operations_total",
			Help: "Количество операций с кэшем",
		},
		[]string{"operation", "result"}, // hit/miss
	)

	// Онлайн пользователи
	OnlineUsers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "pigeongram_online_users",
		Help: "Количество пользователей онлайн",
	})

	// Всего пользователей
	TotalUsers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "pigeongram_total_users",
		Help: "Общее количество зарегистрированных пользователей",
	})

	// Всего сообщений в БД
	TotalMessages = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "pigeongram_total_messages",
		Help: "Общее количество сообщений в базе",
	})
)
