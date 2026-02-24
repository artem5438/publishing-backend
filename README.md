# Publishing Backend

Бэкенд системы заявок «Книжное издательство».
Тема: Малый бизнес №5 — услуги = работы издательства, заявки = заказы на издание книги.

## Стек
- Go (net/http + chi + GORM)
- PostgreSQL 16
- Minio (S3)
- Docker Compose

## Запуск
```bash
docker-compose up -d
go run main.go
