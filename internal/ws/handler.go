package ws

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"

	"addictiveapi/internal/auth"
	"addictiveapi/internal/models"
)

const notificationPollInterval = 10 * time.Second

type Handler struct {
	logger      *slog.Logger
	db          *gorm.DB
	upgrader    websocket.Upgrader
	registry    *Registry
	authService *auth.Service
}

type Message struct {
	Topic   string          `json:"topic"`
	Command string          `json:"command"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type Response struct {
	Topic   string         `json:"topic"`
	Command string         `json:"command"`
	Status  string         `json:"status"`
	Data    map[string]any `json:"data,omitempty"`
	Error   string         `json:"error,omitempty"`
}

func NewHandler(logger *slog.Logger, db *gorm.DB, authService *auth.Service) *Handler {
	registry := NewRegistry()
	registerWebsocketHandlers(registry)

	return &Handler{
		logger:      logger,
		db:          db,
		authService: authService,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		registry: registry,
	}
}

func (h *Handler) ServeWS(c *gin.Context) {
	h.logger.Info("new websocket connection")
	claims, _ := c.Get(claimsContextKey)
	jwtClaims, _ := claims.(*auth.Claims)
	tokenValue, _ := c.Get("wsToken")
	expiresValue, _ := c.Get("wsTokenExpiresAt")
	token, _ := tokenValue.(string)
	expiresAt, _ := expiresValue.(time.Time)

	// If middleware set a desired subprotocol, echo it in the upgrade response so browsers accept it
	var respHeader http.Header
	if sp, ok := c.Get("wsSubprotocol"); ok {
		if s, ok2 := sp.(string); ok2 && s != "" {
			respHeader = http.Header{}
			respHeader.Add("Sec-Websocket-Protocol", s)
		}
	}

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, respHeader)
	if err != nil {
		h.logger.Error("websocket upgrade failed", "error", err)
		return
	}

	session := NewSession(jwtClaims, token, expiresAt)
	context := &Context{Logger: h.logger, Conn: conn, DB: h.db, AuthService: h.authService, Session: session}
	go h.renewJWTLoop(context)
	go h.notificationLoop(context)

	defer conn.Close()
	defer session.Close()

	if jwtClaims != nil {
		_ = conn.WriteJSON(Response{
			Topic:   "system",
			Command: "connected",
			Status:  "ok",
			Data: map[string]any{
				"user_id": jwtClaims.UserID,
				"email":   jwtClaims.Email,
			},
		})
	}

	for {
		var message Message
		if err := conn.ReadJSON(&message); err != nil {
			if errors.Is(err, websocket.ErrCloseSent) {
				return
			}

			h.logger.Info("websocket closed", "error", err)
			return
		}

		response := h.registry.Dispatch(context, message)
		if err := context.WriteJSON(response); err != nil {
			h.logger.Info("websocket write failed", "error", err)
			return
		}
	}
}

func (h *Handler) renewJWTLoop(ctx *Context) {
	if ctx == nil || ctx.Session == nil || ctx.Session.Claims == nil || ctx.AuthService == nil {
		return
	}

	for {
		_, expiresAt := ctx.Session.Snapshot()
		if expiresAt.IsZero() {
			return
		}

		delay := time.Until(expiresAt)
		if delay < 0 {
			delay = 0
		}

		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
			newToken, renewedAt, err := ctx.AuthService.RenewToken(ctx.Session.Claims)
			if err != nil {
				ctx.Logger.Error("jwt renewal failed", "error", err)
				timer.Stop()
				return
			}

			ctx.Session.UpdateRenewal(newToken, renewedAt)
			if err := ctx.WriteJSON(Response{
				Topic:   TopicJWT,
				Command: CommandRenew,
				Status:  "ok",
				Data: map[string]any{
					"token":      newToken,
					"expires_at": renewedAt.UTC().Format(time.RFC3339),
				},
			}); err != nil {
				ctx.Logger.Info("jwt renewal send failed", "error", err)
				timer.Stop()
				return
			}
			timer.Stop()
		case <-ctx.Session.Done():
			timer.Stop()
			return
		}
	}
}

func (h *Handler) notificationLoop(ctx *Context) {
	if ctx == nil || ctx.Session == nil || ctx.Session.Claims == nil || ctx.DB == nil {
		return
	}

	if err := h.pushPendingNotifications(ctx); err != nil {
		ctx.Logger.Info("notification initial check failed", "error", err)
		return
	}

	ticker := time.NewTicker(notificationPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := h.pushPendingNotifications(ctx); err != nil {
				ctx.Logger.Info("notification check failed", "error", err)
				return
			}
		case <-ctx.Session.Done():
			return
		}
	}
}

func (h *Handler) pushPendingNotifications(ctx *Context) error {
	userID := ctx.UserID()
	if userID == 0 {
		return nil
	}

	var notifications []models.Notification
	if err := ctx.DB.Where("user_id = ? AND delivered_at IS NULL", userID).
		Order("created_at ASC").
		Find(&notifications).Error; err != nil {
		return err
	}

	for _, n := range notifications {
		if err := ctx.WriteJSON(Response{
			Topic:   TopicNotification,
			Command: "push",
			Status:  "ok",
			Data: map[string]any{
				"id":         n.ID,
				"title":      n.Title,
				"content":    n.Content,
				"style":      n.Style,
				"created_at": n.CreatedAt.UTC().Format(time.RFC3339),
			},
		}); err != nil {
			return err
		}

		now := time.Now().UTC()
		if err := ctx.DB.Model(&models.Notification{}).Where("id = ?", n.ID).Update("delivered_at", now).Error; err != nil {
			return err
		}
	}

	return nil
}
