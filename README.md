# Team Task REST API

REST API для управления задачами команд с ролевым доступом, JWT, историей изменений, комментариями, SQL-отчетом и Redis-кешем. Слои приложения разделены на `domain`, `application`, `infrastructure` и `presentation`.

## Запуск

Требуются Docker и Docker Compose. Для копирования токена в примерах запросов также нужен `jq`. Скопируйте `.env.example` в `.env`, замените пароли MySQL/Redis и `JWT_SECRET` длинными случайными значениями, затем запустите:

```sh
docker compose up --build
```

При старте приложение применяет схему MySQL. Проверка готовности: `GET http://localhost:8080/health`. OpenAPI-спецификация: `GET http://localhost:8080/openapi.yaml`.

## API

- `POST /api/v1/register` — `{ "email", "password", "name" }`; пароль не короче 8 символов.
- `POST /api/v1/login` — `{ "email", "password" }`; возвращает JWT.
- `POST /api/v1/teams` — `{ "name" }`; создатель автоматически становится `owner`.
- `GET /api/v1/teams` — команды текущего пользователя.
- `POST /api/v1/teams/{id}/invite` — `{ "user_id", "role": "member"|"admin" }`; owner/admin.
- `PUT /api/v1/teams/{id}/members/{user_id}` — `{ "role": "member"|"admin" }`; только owner.
- `DELETE /api/v1/teams/{id}/members/{user_id}` — owner/admin; создателя команды удалить нельзя.
- `POST /api/v1/tasks` — `{ "team_id", "title", "description", "status", "assignee_id" }`; статус по умолчанию `todo`.
- `GET /api/v1/tasks?team_id=1&status=todo&assignee_id=5&limit=20&offset=0` — список только для участника команды.
- `PUT /api/v1/tasks/{id}` — передайте `version` и изменяемые поля; `assignee_id: 0` снимает исполнителя. Ответ `409` означает устаревшую версию.
- `GET /api/v1/tasks/{id}/history` — история с автором и diff старых/новых значений.
- `POST /api/v1/tasks/{id}/comments` — `{ "content" }`.
- `GET /api/v1/tasks/{id}/comments` — комментарии задачи.
- `GET /api/v1/teams/{team_id}/stats` — отчет только для owner/admin этой команды.

Для всех маршрутов, кроме регистрации, входа и health check, требуется заголовок `Authorization: Bearer <JWT>`.

### Примеры запросов

Регистрация и получение токена:

```sh
curl -X POST http://localhost:8080/api/v1/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","password":"correct-horse-42","name":"Alice"}'

TOKEN=$(curl -sS -X POST http://localhost:8080/api/v1/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","password":"correct-horse-42"}' | jq -r .token)
```

Создание команды, задачи и получение списка:

```sh
TEAM_ID=$(curl -sS -X POST http://localhost:8080/api/v1/teams \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Platform"}' | jq -r .id)

curl -X POST http://localhost:8080/api/v1/tasks \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"team_id\":$TEAM_ID,\"title\":\"Prepare release\",\"description\":\"Release checklist\",\"status\":\"todo\"}"

curl "http://localhost:8080/api/v1/tasks?team_id=$TEAM_ID&status=todo&limit=20&offset=0" \
  -H "Authorization: Bearer $TOKEN"
```

Для примеров остальных операций и схем запросов/ответов откройте Swagger/OpenAPI по адресу `/openapi.yaml`.

## Доступ и надежность

`owner` и `admin` редактируют все задачи команды; `member` — созданные им задачи и задачи, назначенные ему. Исполнитель может менять только статус. Исполнитель всегда должен состоять в команде. Доступ к истории и комментариям требует членства в команде.

Изменение задачи и запись истории выполняются одной транзакцией. `version` проверяется под `SELECT ... FOR UPDATE`, поэтому конкурентное устаревшее обновление получает `409`. Создание команды и назначение owner также атомарны. Список задач кешируется в Redis на 5 минут по команде и всем фильтрам; создание/обновление задачи инвалидирует кеш команды. Если Redis не принимает инвалидацию после успешной записи в MySQL, операция остается успешной, ошибка записывается в лог, а кеш истекает по TTL.

Статистика выполняется одним MySQL 8 CTE-запросом и возвращает количества задач по статусам, трех лучших исполнителей по завершенным за 30 дней задачам, среднее время закрытия и количество комментариев на каждую задачу. Интеграционный тест отчета можно запустить с отдельной тестовой MySQL-базой через переменную окружения `MYSQL_TEST_DSN`; без нее тест пропускается.

## Конфигурация

Все параметры задаются переменными окружения: `HTTP_ADDR`, `MYSQL_DSN`, `REDIS_ADDR`, `REDIS_PASSWORD`, `REDIS_DB`, `JWT_SECRET`, `JWT_TTL`. В Docker Compose параметры служб и учетные данные задаются в `.env`; секрет JWT должен содержать не менее 32 символов.
