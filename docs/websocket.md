# Websocket Contract

The websocket endpoint is `GET /ws`.

## Auth

Send the JWT in the upgrade request header.

```http
Authorization: Bearer <jwt>
```

If the token expires while the socket is open, the server sends a renewal event automatically.

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

Note: browsers cannot set custom headers on the WebSocket handshake. To connect from a browser, include the token as a query parameter:

```js
// Browser example
const ws = new WebSocket(`wss://example.com/ws?token=${encodeURIComponent(jwt)}`)
```

Alternatively you can send the token using the `Sec-WebSocket-Protocol` subprotocols.

```js
// send the token as the second subprotocol value; server will echo 'token'
const ws = new WebSocket('wss://example.com/ws', ['token', jwt])
```

The server will echo the chosen subprotocol so the browser accepts the upgrade.

## Message Shape

```json
{
  "topic": "friends",
  "command": "add",
  "payload": {
    "user_id": 42
  }
}
```

## Topics

- 🧩 `system/ping` - socket health check
- 🔁 `jwt/renew` - automatic token renewal event
- 🔔 `notification/push` - server push from database-backed notifications
- 👥 `friends/add`, `friends/remove`, `friends/requests`
- ➕ `follows/follow`, `follows/unfollow`
- 🏅 `score/publish`, `score/friend_scores`, `score/followed_scores`
- 🛡️ `profile/public` - set profile visibility

## Notification Push Service

Notifications are pushed automatically per connected user.

- The server checks once when the websocket connection is established.
- It then checks every 10 seconds.
- If a database notification exists for the connected user, a websocket event is sent with `title`, `content`, and `style`.

Notification event shape:

```json
{
  "topic": "notification",
  "command": "push",
  "status": "ok",
  "data": {
    "id": 101,
    "title": "Daily Challenge",
    "content": "A new challenge is available.",
    "style": "info",
    "created_at": "2026-05-26T13:00:00Z"
  }
}
```

## Score Privacy Rules

- Friends can see each other's score even when a profile is private.
- Follow-only users cannot see a private profile's score.
- For redacted scores, the response includes `score_redacted: true` and omits `score`.

## Payloads

Friend and follow commands accept either `user_id` or `email`.

```json
{ "user_id": 42 }
```

Score updates are a single number.

```json
{ "score": 1200 }
```

## Example: Publish a Score

Send a websocket message like this:

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

Profile visibility is a boolean.

```json
{ "public": true }
```

## How To Use Commands

All commands are sent as JSON messages on the same websocket connection.

Base message format:

```json
{
  "topic": "<topic>",
  "command": "<command>",
  "payload": { ... }
}
```

### 1. Health Check

Request:

```json
{
  "topic": "system",
  "command": "ping"
}
```

Response:

```json
{
  "topic": "system",
  "command": "ping",
  "status": "ok",
  "data": {
    "timestamp": "2026-05-26T12:00:00Z"
  }
}
```

### 2. Update Your Score

Request:

```json
{
  "topic": "score",
  "command": "publish",
  "payload": {
    "score": 1200
  }
}
```

Response:

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

### 3. Read Friends' Scores

Request:

```json
{
  "topic": "score",
  "command": "friend_scores",
  "payload": {}
}
```

Response (friends always include score):

```json
{
  "topic": "score",
  "command": "friend_scores",
  "status": "ok",
  "data": {
    "users": [
      {
        "user_id": 2,
        "email": "friend@example.com",
        "profile_public": false,
        "score": 980
      }
    ]
  }
}
```

### 4. Read Followed Users' Scores

Request:

```json
{
  "topic": "score",
  "command": "followed_scores",
  "payload": {}
}
```

Response (private + not friend => redacted):

```json
{
  "topic": "score",
  "command": "followed_scores",
  "status": "ok",
  "data": {
    "users": [
      {
        "user_id": 3,
        "email": "followed@example.com",
        "profile_public": false,
        "score_redacted": true
      }
    ]
  }
}
```

### 5. Change Profile Visibility

Request:

```json
{
  "topic": "profile",
  "command": "public",
  "payload": {
    "public": true
  }
}
```

Response:

```json
{
  "topic": "profile",
  "command": "public",
  "status": "ok",
  "data": {
    "public": true
  }
}
```

### 6. Friends And Follows

Use `user_id` or `email` in payload.

Add friend:

```json
{
  "topic": "friends",
  "command": "add",
  "payload": {
    "user_id": 42
  }
}
```

First `friends/add` creates a pending request.
If the other user also sends `friends/add` back, the request becomes accepted.

Example `friends/add` response:

```json
{
  "topic": "friends",
  "command": "add",
  "status": "ok",
  "data": {
    "user_id": 42,
    "friend_status": "pending"
  }
}
```

On reciprocal add, `friend_status` becomes `accepted`.

Remove friend:

```json
{
  "topic": "friends",
  "command": "remove",
  "payload": {
    "email": "user@example.com"
  }
}
```

Follow user:

```json
{
  "topic": "follows",
  "command": "follow",
  "payload": {
    "user_id": 42
  }
}
```

Unfollow user:

```json
{
  "topic": "follows",
  "command": "unfollow",
  "payload": {
    "email": "user@example.com"
  }
}
```

Success response shape:

```json
{
  "topic": "friends",
  "command": "add",
  "status": "ok",
  "data": {
    "user_id": 42
  }
}
```

Check requests:

```json
{
  "topic": "friends",
  "command": "requests",
  "payload": {}
}
```

Response:

```json
{
  "topic": "friends",
  "command": "requests",
  "status": "ok",
  "data": {
    "incoming": [
      {
        "request_id": 12,
        "from_user_id": 7,
        "from_email": "alice@example.com",
        "requested_at": "2026-05-26T12:00:00Z",
        "friend_status": "pending"
      }
    ],
    "outgoing": [
      {
        "request_id": 15,
        "to_user_id": 9,
        "to_email": "bob@example.com",
        "requested_at": "2026-05-26T12:05:00Z",
        "friend_status": "pending"
      }
    ]
  }
}
```