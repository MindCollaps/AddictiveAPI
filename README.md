# AddictiveAPI

AddictiveAPI is a small Go backend for user auth, social features, scores, and realtime websocket messaging.

It uses:

- Gin for HTTP and websocket routing
- SQLite with Gorm for persistence
- JWTs for login and websocket access
- Docker for local and containerized runs

## What It Does

- 🔐 Register users and log in with hashed passwords
- 🌐 Issue JWTs over HTTP for authenticated access
- 💬 Keep a websocket open at `/ws` for realtime commands
- 👥 Manage friends and follow relationships
- 🏅 Publish and read scores
- 🧭 Control profile visibility with a public/private flag

## Run It

Local:

```bash
go mod tidy
go run ./cmd/api
```

Docker:

```bash
docker compose up --build
```

## HTTP Auth Flow

1. Register a user with `POST /api/v1/auth/register`.
2. Log in with `POST /api/v1/auth/login`.
3. Copy the returned JWT.
4. Send that JWT in the websocket upgrade request header.

Example login response:

```json
{
	"token": "<jwt>",
	"expires_at": "2026-05-29T12:00:00Z",
	"user": {
		"id": 1,
		"email": "user@example.com"
	}
}
```

## Websockets

Connect to `GET /ws`.

Send the JWT as a header when opening the socket:

```http
Authorization: Bearer <jwt>
```

If the token expires while the socket is open, the server sends a renewal event automatically:

```json
{
	"topic": "jwt",
	"command": "renew",
	"status": "ok",
	"data": {
		"token": "<new-jwt>",
		"expires_at": "2026-05-29T12:00:00Z"
	}
}
```

Note: browsers cannot set arbitrary headers on the WebSocket handshake. Two browser-compatible options:

- Query string (simple but tokens may appear in logs/referrers):

```js
const ws = new WebSocket(`wss://example.com/ws?token=${encodeURIComponent(jwt)}`)
```

- Sec-WebSocket-Protocol subprotocol (server echoes the chosen protocol):

```js
// send the token as the second subprotocol value
const ws = new WebSocket('wss://example.com/ws', ['token', jwt])
```

### Message Shape

```json
{
	"topic": "friends",
	"command": "add",
	"payload": {
		"user_id": 42
	}
}
```

### Topics

- 🧩 `system/ping` - quick socket health check
- 🔁 `jwt/renew` - automatic JWT renewal event
- 👥 `friends/*` - add, remove, list requests, accept, decline
- ➕ `follows/*` - follow, unfollow, list following, list followers
- 🏅 `score/*` - publish score, get friend scores, get followed scores
- 🛡️ `profile/public` - toggle public profile visibility

Score privacy rule:

- Friends can see score even when the target profile is private.
- Follow-only users cannot see score when the target profile is private.
- Redacted responses include `score_redacted: true` and omit `score`.

### Payload Examples

Friend and follow commands accept `user_id` or `email`.

```json
{ "user_id": 42 }
```

Score updates are just a number:

```json
{ "score": 1200 }
```

Example score publish message:

```json
{
	"topic": "score",
	"command": "publish",
	"payload": {
		"score": 1200
	}
}
```

Expected response:

```json
{
	"topic": "score",
	"command": "publish",
	"status": "ok",
	"data": {
		"score": 1200
	}
}
```

Profile visibility is a boolean:

```json
{ "public": true }
```

## More Details

See [docs/websocket.md](docs/websocket.md) for the full websocket contract and message examples.