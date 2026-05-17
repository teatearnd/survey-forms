Survey Forms API
================

Survey management service with possibility to create questionnaires, submit answers and gather data that becomes open and reusable for everyone.
Go 1.26.1 and a Redis server is required. SQLite is used as a database, test suites make temporary db files and will clean up after. JWT authentication is for protected submission and cart endpoints. Cart items are stored as JSON in Redis under keys prefixed with `cart:<user_id>`

Getting started
---------------

1. Clone the repository.
2. Build or run with go tools:

```bash
go build -o survey-service .
./survey-service
# or
go run main.go
```

3. The server starts on `:8080` by default.

Docker
------

Use Docker Compose to start the app and Redis together:

```bash
docker compose up --build
```

`JWT_SECRET`, `JWT_ISSUER`, and `JWT_AUDIENCE` are expected in the environment. See `.env.example`.

Environment variables
---------------------
Required for JWT configuration:

- `JWT_SECRET` — HMAC secret used to validate tokens
- `JWT_ISSUER` — expected token issuer
- `JWT_AUDIENCE` — expected token audience

Optional for Redis:

- `REDIS_ADDRESS` — Redis server address (default: `localhost:6379`)


Project structure
-----------------

- `main.go` — application entrypoint and HTTP route definitions
- `internal/handlers` — HTTP handlers (create survey, list surveys, submissions)
- `internal/repository` — database access and schema
- `internal/models` — domain models (Survey, Question, Submission)
- `internal/dto` — request/response DTOs and conversions
- `internal/auth` — JWT token validation and claims
- `internal/validations` — request decoding and validation helpers
- `internal/testutil` — Utilities for test suites (setting up test db, Redis, etc)

Endpoints
---

- `GET /` — health / placeholder
- `GET /surveys` — list surveys
	- Response: array of surveys `{ id, name, description, created_at }`
- `POST /survey` — create a survey
	- Body: `RequestCreateSurvey` (see `internal/dto`)
	- Response: created survey object and message (201)
- `GET /survey/{surveyId}` — retrieve a single survey by id
- `DELETE /survey/{surveyId}` — delete a survey by id
- `GET /catalog/surveys/{surveyId}/submissions` — list public submissions for a survey (anonymous)

**Survey Submissions**

- `POST /survey/{surveyId}/submissions` — submit answers for a survey
	- Body: `RequestCreateSubmission` (answers array with `question_id`, optional `choice_id`, optional `text_response`)
	- Requires a valid JWT; token's `user_id` is used as the submitter
	- Submissions are public by default for the catalog
	- Response: created submission (201)
- `GET /survey/{surveyId}/submissions` — list submissions for a survey
	- Admin users (token claim `role` == `admin`) can retrieve all submissions; otherwise only submissions for the token user are returned
- `GET /users/{userId}/submissions` — list submissions for a user
	- Admins or the user themself may access this endpoint

**Shopping Cart** (stored in Redis)

- `POST /cart/items` — add an item to user's cart
	- Body: `{ "item": { "survey_id", "question_id", "submission_id" (optional), "answer_id" (optional), "note" (optional) } }`
	- Response: 201 Created
- `GET /cart` — retrieve user's cart items
	- Query parameters: `limit` (default 50), `offset` (default 0) for pagination
	- Response: array of cart items
- `DELETE /cart/items/{index}` — remove item at specific index from cart
	- Response: 200 OK
- `DELETE /cart` — clear entire user's cart
	- Response: 200 OK
- `GET /survey/{surveyId}/submissions` — list submissions for a survey
	- Admin users (token claim `role` == `admin`) can retrieve all submissions; otherwise only submissions for the token user are returned
- `GET /users/{userId}/submissions` — list submissions for a user
	- Admins or the user themself may access this endpoint

Authentication
--------------

Protect endpoints by including a JWT in the `Authorization` header:

```
Authorization: Bearer <token>
```

The token should contain claims compatible with `internal/auth.AccessClaims` (email, user_id, role).

Examples
--------

Create a survey (example):

```bash
curl -X POST http://localhost:8080/survey \
	-H "Content-Type: application/json" \
	-d '{
		"name": "Example",
		"description": "Example survey",
		"questions_list": [
			{"description":"How are you?","type":1,"is_mandatory":false}
		]
	}'
```

Submit answers (example, protected):

```bash
curl -X POST http://localhost:8080/survey/<survey-id>/submissions \
	-H "Authorization: Bearer $TOKEN" \
	-H "Content-Type: application/json" \
	-d '{"answers":[{"question_id":"<q-id>","text_response":"Good"}]}'
```

Public catalog (examples):

```bash
# List public submissions for a survey (first 50)
curl http://localhost:8080/catalog/surveys/<survey-id>/submissions

# List public submissions with pagination
curl http://localhost:8080/catalog/surveys/<survey-id>/submissions?limit=100&offset=0

# List public answers for a question
curl http://localhost:8080/catalog/questions/<question-id>/answers?limit=100&offset=0
