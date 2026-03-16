Создаем systemd сервис для автоматического обновления (на хосте)

/etc/systemd/system/minio-token.service
/etc/systemd/system/minio-token.timer

Запускаем автоматическое обновление
bash

# Активируем таймер
sudo systemctl daemon-reload
sudo systemctl enable minio-token.timer
sudo systemctl start minio-token.timer

# Проверить статус
sudo systemctl status minio-token.timer
sudo systemctl list-timers | grep minio

Ручное обновление токена (для теста)
bash

cd docker/monitoring
./scripts/update-minio-token.sh

Проверка работы
bash

# Проверить наличие токена
cat docker/monitoring/minio-token

# Проверить логи обновления
docker logs pigeongram_prometheus --tail 50

# Проверить статус MinIO target в Prometheus
curl http://localhost:9090/api/v1/targets | jq '.data.activeTargets[] | select(.jobName=="minio")'