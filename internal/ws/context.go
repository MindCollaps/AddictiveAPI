package ws

import (
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"gorm.io/gorm"

	"addictiveapi/internal/auth"
)

type Context struct {
	Logger      *slog.Logger
	Conn        *websocket.Conn
	DB          *gorm.DB
	AuthService *auth.Service
	Session     *Session
	writeMu     sync.Mutex
}

func (c *Context) CurrentToken() (string, time.Time) {
	return c.Session.Snapshot()
}

func (c *Context) WriteJSON(value any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	return c.Conn.WriteJSON(value)
}

func (c *Context) UserID() uint {
	if c.Session == nil || c.Session.Claims == nil {
		return 0
	}

	return c.Session.Claims.UserID
}
