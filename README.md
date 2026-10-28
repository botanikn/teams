## Запуск

Требуются Docker и Docker Compose. Скопируйте `.env.example` в `.env`, замените пароли MySQL/Redis и `JWT_SECRET` длинными случайными значениями, затем запустите:

```sh
docker compose up --build
```

При старте приложение применяет схему MySQL. Проверка готовности: `GET http://localhost:8080/health`. OpenAPI-спецификация: `GET http://localhost:8080/openapi.yaml`.