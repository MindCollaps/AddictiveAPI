package ws

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"addictiveapi/internal/models"
)

const (
	TopicSystem  = "system"
	TopicJWT     = "jwt"
	TopicNotification = "notification"
	TopicFriends = "friends"
	TopicFollows = "follows"
	TopicScore   = "score"
	TopicProfile = "profile"

	CommandPing           = "ping"
	CommandRenew          = "renew"
	CommandPublish        = "publish"
	CommandFriendScores   = "friend_scores"
	CommandFollowedScores = "followed_scores"
	CommandPublic         = "public"
	CommandAdd            = "add"
	CommandRemove         = "remove"
	CommandRequests       = "requests"
	CommandFollow         = "follow"
	CommandUnfollow       = "unfollow"
)

func registerWebsocketHandlers(registry *Registry) {
	registry.Register(TopicSystem, CommandPing, handleSystemPing)
	registry.Register(TopicScore, CommandPublish, handleScorePublish)
	registry.Register(TopicScore, CommandFriendScores, handleFriendScores)
	registry.Register(TopicScore, CommandFollowedScores, handleFollowedScores)
	registry.Register(TopicProfile, CommandPublic, handleProfilePublic)
	registry.Register(TopicFriends, CommandAdd, handleFriendAdd)
	registry.Register(TopicFriends, CommandRemove, handleFriendRemove)
	registry.Register(TopicFriends, CommandRequests, handleFriendRequests)
	registry.Register(TopicFollows, CommandFollow, handleFollow)
	registry.Register(TopicFollows, CommandUnfollow, handleUnfollow)
}

type userRefPayload struct {
	UserID uint   `json:"user_id"`
	Email  string `json:"email"`
}

type scorePayload struct {
	Score int64 `json:"score"`
}

type profilePublicPayload struct {
	Public bool `json:"public"`
}

func handleSystemPing(_ *Context, message Message) Response {
	return Response{
		Topic:   message.Topic,
		Command: message.Command,
		Status:  "ok",
		Data: map[string]any{
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		},
	}
}

func handleScorePublish(ctx *Context, message Message) Response {
	var payload scorePayload
	if err := decodePayload(message.Payload, &payload); err != nil {
		return errorResponse(message, "invalid payload")
	}

	user, err := currentUser(ctx)
	if err != nil {
		return errorResponse(message, err.Error())
	}

	user.Score = payload.Score
	if err := ctx.DB.Model(&models.User{}).Where("id = ?", user.ID).Update("score", payload.Score).Error; err != nil {
		return errorResponse(message, "failed to update score")
	}

	return Response{
		Topic:   message.Topic,
		Command: message.Command,
		Status:  "ok",
		Data: map[string]any{
			"score": payload.Score,
		},
	}
}

func handleFriendScores(ctx *Context, message Message) Response {
	userID := ctx.UserID()
	if userID == 0 {
		return errorResponse(message, "unauthorized")
	}

	friendIDs, err := friendIDsFor(ctx.DB, userID)
	if err != nil {
		return errorResponse(message, "failed to load friends")
	}

	users, err := loadUsersByIDs(ctx.DB, friendIDs)
	if err != nil {
		return errorResponse(message, "failed to load friend scores")
	}

	items := make([]map[string]any, 0, len(users))
	for _, u := range users {
		// Friends can always see scores, even if profile is private.
		items = append(items, map[string]any{
			"user_id":        u.ID,
			"email":          u.Email,
			"profile_public": u.ProfilePublic,
			"score":          u.Score,
		})
	}

	return Response{
		Topic:   message.Topic,
		Command: message.Command,
		Status:  "ok",
		Data: map[string]any{
			"users": items,
		},
	}
}

func handleFollowedScores(ctx *Context, message Message) Response {
	userID := ctx.UserID()
	if userID == 0 {
		return errorResponse(message, "unauthorized")
	}

	followedIDs, err := followedIDsFor(ctx.DB, userID)
	if err != nil {
		return errorResponse(message, "failed to load follows")
	}

	users, err := loadUsersByIDs(ctx.DB, followedIDs)
	if err != nil {
		return errorResponse(message, "failed to load followed users")
	}

	friendSet := map[uint]struct{}{}
	friendIDs, err := friendIDsFor(ctx.DB, userID)
	if err != nil {
		return errorResponse(message, "failed to load friends")
	}
	for _, id := range friendIDs {
		friendSet[id] = struct{}{}
	}

	items := make([]map[string]any, 0, len(users))
	for _, u := range users {
		_, isFriend := friendSet[u.ID]
		canSeeScore := u.ProfilePublic || isFriend

		item := map[string]any{
			"user_id":        u.ID,
			"email":          u.Email,
			"profile_public": u.ProfilePublic,
		}

		if canSeeScore {
			item["score"] = u.Score
		} else {
			item["score_redacted"] = true
		}

		items = append(items, item)
	}

	return Response{
		Topic:   message.Topic,
		Command: message.Command,
		Status:  "ok",
		Data: map[string]any{
			"users": items,
		},
	}
}

func handleProfilePublic(ctx *Context, message Message) Response {
	var payload profilePublicPayload
	if err := decodePayload(message.Payload, &payload); err != nil {
		return errorResponse(message, "invalid payload")
	}

	userID := ctx.UserID()
	if userID == 0 {
		return errorResponse(message, "unauthorized")
	}

	if err := ctx.DB.Model(&models.User{}).Where("id = ?", userID).Update("profile_public", payload.Public).Error; err != nil {
		return errorResponse(message, "failed to update profile visibility")
	}

	return Response{
		Topic:   message.Topic,
		Command: message.Command,
		Status:  "ok",
		Data: map[string]any{
			"public": payload.Public,
		},
	}
}

func handleFriendAdd(ctx *Context, message Message) Response {
	targetID, err := resolveTargetUserID(ctx, message.Payload)
	if err != nil {
		return errorResponse(message, err.Error())
	}

	selfID := ctx.UserID()
	if selfID == 0 {
		return errorResponse(message, "unauthorized")
	}
	if targetID == selfID {
		return errorResponse(message, "cannot friend yourself")
	}

	selfUser, err := currentUser(ctx)
	if err != nil {
		return errorResponse(message, err.Error())
	}

	var targetUser models.User
	if err := ctx.DB.First(&targetUser, targetID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errorResponse(message, "user not found")
		}
		return errorResponse(message, "failed to load target user")
	}

	var state string
	err = ctx.DB.Transaction(func(tx *gorm.DB) error {
		var existing models.FriendRequest
		err := tx.Where("((requester_id = ? AND addressee_id = ?) OR (requester_id = ? AND addressee_id = ?))", selfID, targetID, targetID, selfID).
			First(&existing).Error

		if errors.Is(err, gorm.ErrRecordNotFound) {
			pending := models.FriendRequest{RequesterID: selfID, AddresseeID: targetID, Status: "pending"}
			if err := tx.Create(&pending).Error; err != nil {
				return err
			}
			if err := createNotification(
				tx,
				targetID,
				"Friend Request",
				fmt.Sprintf("A friend request from %s has been received.", selfUser.Username),
				"friend_request",
			); err != nil {
				return err
			}
			state = "pending"
			return nil
		}

		if err != nil {
			return err
		}

		if existing.Status == "accepted" {
			state = "accepted"
			return nil
		}

		// Reciprocal add accepts the pending request.
		if existing.Status == "pending" && existing.RequesterID == targetID && existing.AddresseeID == selfID {
			if err := tx.Model(&models.FriendRequest{}).Where("id = ?", existing.ID).Update("status", "accepted").Error; err != nil {
				return err
			}
			if err := createNotification(
				tx,
				targetID,
				"Friend Request Accepted",
				fmt.Sprintf("Your friend request was accepted by %s.", selfUser.Username),
				"default",
			); err != nil {
				return err
			}
			if err := createNotification(
				tx,
				selfID,
				"Friend Request Accepted",
				fmt.Sprintf("You are now friends with %s.", targetUser.Username),
				"default",
			); err != nil {
				return err
			}
			state = "accepted"
			return nil
		}

		// Request already sent by current user and still pending.
		state = "pending"
		return nil
	})
	if err != nil {
		return errorResponse(message, "failed to process friend request")
	}

	return Response{Topic: message.Topic, Command: message.Command, Status: "ok", Data: map[string]any{"user_id": targetID, "friend_status": state}}
}

func handleFriendRequests(ctx *Context, message Message) Response {
	userID := ctx.UserID()
	if userID == 0 {
		return errorResponse(message, "unauthorized")
	}

	var requests []models.FriendRequest
	if err := ctx.DB.Where("status = ? AND (requester_id = ? OR addressee_id = ?)", "pending", userID, userID).
		Order("created_at DESC").
		Find(&requests).Error; err != nil {
		return errorResponse(message, "failed to load friend requests")
	}

	userIDs := make([]uint, 0, len(requests)*2)
	seen := map[uint]struct{}{}
	for _, r := range requests {
		if _, ok := seen[r.RequesterID]; !ok {
			seen[r.RequesterID] = struct{}{}
			userIDs = append(userIDs, r.RequesterID)
		}
		if _, ok := seen[r.AddresseeID]; !ok {
			seen[r.AddresseeID] = struct{}{}
			userIDs = append(userIDs, r.AddresseeID)
		}
	}

	emailsByID := map[uint]string{}
	if len(userIDs) > 0 {
		var users []models.User
		if err := ctx.DB.Where("id IN ?", userIDs).Find(&users).Error; err != nil {
			return errorResponse(message, "failed to resolve request users")
		}
		for _, u := range users {
			emailsByID[u.ID] = u.Email
		}
	}

	incoming := make([]map[string]any, 0)
	outgoing := make([]map[string]any, 0)
	for _, r := range requests {
		if r.AddresseeID == userID {
			incoming = append(incoming, map[string]any{
				"request_id":    r.ID,
				"from_user_id":  r.RequesterID,
				"from_email":    emailsByID[r.RequesterID],
				"requested_at":  r.CreatedAt.UTC().Format(time.RFC3339),
				"friend_status": r.Status,
			})
			continue
		}

		outgoing = append(outgoing, map[string]any{
			"request_id":    r.ID,
			"to_user_id":    r.AddresseeID,
			"to_email":      emailsByID[r.AddresseeID],
			"requested_at":  r.CreatedAt.UTC().Format(time.RFC3339),
			"friend_status": r.Status,
		})
	}

	return Response{
		Topic:   message.Topic,
		Command: message.Command,
		Status:  "ok",
		Data: map[string]any{
			"incoming": incoming,
			"outgoing": outgoing,
		},
	}
}

func handleFriendRemove(ctx *Context, message Message) Response {
	targetID, err := resolveTargetUserID(ctx, message.Payload)
	if err != nil {
		return errorResponse(message, err.Error())
	}

	selfID := ctx.UserID()
	if selfID == 0 {
		return errorResponse(message, "unauthorized")
	}

	selfUser, err := currentUser(ctx)
	if err != nil {
		return errorResponse(message, err.Error())
	}

	var targetUser models.User
	if err := ctx.DB.First(&targetUser, targetID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errorResponse(message, "user not found")
		}
		return errorResponse(message, "failed to load target user")
	}

	err = ctx.DB.Transaction(func(tx *gorm.DB) error {
		var existing models.FriendRequest
		err := tx.Where("((requester_id = ? AND addressee_id = ?) OR (requester_id = ? AND addressee_id = ?))", selfID, targetID, targetID, selfID).
			First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}

		if err := tx.Where("id = ?", existing.ID).Delete(&models.FriendRequest{}).Error; err != nil {
			return err
		}

		// Decline case: receiver removes an incoming pending request.
		if existing.Status == "pending" && existing.RequesterID == targetID && existing.AddresseeID == selfID {
			if err := createNotification(
				tx,
				targetID,
				"Friend Request Declined",
				fmt.Sprintf("Your friend request to %s was declined.", selfUser.Username),
				"default",
			); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return errorResponse(message, "failed to remove friend")
	}

	return Response{Topic: message.Topic, Command: message.Command, Status: "ok", Data: map[string]any{"user_id": targetID, "email": targetUser.Email}}
}

func handleFollow(ctx *Context, message Message) Response {
	targetID, err := resolveTargetUserID(ctx, message.Payload)
	if err != nil {
		return errorResponse(message, err.Error())
	}

	selfID := ctx.UserID()
	if selfID == 0 {
		return errorResponse(message, "unauthorized")
	}
	if targetID == selfID {
		return errorResponse(message, "cannot follow yourself")
	}

	follow := models.Follow{FollowerID: selfID, FolloweeID: targetID}
	if err := ctx.DB.Where("follower_id = ? AND followee_id = ?", selfID, targetID).FirstOrCreate(&follow).Error; err != nil {
		return errorResponse(message, "failed to follow user")
	}

	return Response{Topic: message.Topic, Command: message.Command, Status: "ok", Data: map[string]any{"user_id": targetID}}
}

func handleUnfollow(ctx *Context, message Message) Response {
	targetID, err := resolveTargetUserID(ctx, message.Payload)
	if err != nil {
		return errorResponse(message, err.Error())
	}

	selfID := ctx.UserID()
	if selfID == 0 {
		return errorResponse(message, "unauthorized")
	}

	if err := ctx.DB.Where("follower_id = ? AND followee_id = ?", selfID, targetID).Delete(&models.Follow{}).Error; err != nil {
		return errorResponse(message, "failed to unfollow user")
	}

	return Response{Topic: message.Topic, Command: message.Command, Status: "ok", Data: map[string]any{"user_id": targetID}}
}

func errorResponse(message Message, errText string) Response {
	return Response{Topic: message.Topic, Command: message.Command, Status: "error", Error: errText}
}

func decodePayload(raw json.RawMessage, out any) error {
	if len(raw) == 0 {
		return errors.New("missing payload")
	}
	return json.Unmarshal(raw, out)
}

func currentUser(ctx *Context) (*models.User, error) {
	userID := ctx.UserID()
	if userID == 0 {
		return nil, errors.New("unauthorized")
	}

	var user models.User
	if err := ctx.DB.First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, errors.New("failed to load user")
	}

	return &user, nil
}

func resolveTargetUserID(ctx *Context, raw json.RawMessage) (uint, error) {
	var payload userRefPayload
	if err := decodePayload(raw, &payload); err != nil {
		return 0, err
	}

	if payload.UserID != 0 {
		return payload.UserID, nil
	}

	email := strings.TrimSpace(strings.ToLower(payload.Email))
	if email == "" {
		return 0, errors.New("user_id or email required")
	}

	var user models.User
	if err := ctx.DB.Where("email = ?", email).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, fmt.Errorf("user not found")
		}
		return 0, errors.New("failed to resolve user")
	}

	return user.ID, nil
}

func friendIDsFor(db *gorm.DB, userID uint) ([]uint, error) {
	var requests []models.FriendRequest
	if err := db.Where("status = ? AND (requester_id = ? OR addressee_id = ?)", "accepted", userID, userID).Find(&requests).Error; err != nil {
		return nil, err
	}

	ids := make([]uint, 0, len(requests))
	seen := map[uint]struct{}{}
	for _, fr := range requests {
		var other uint
		if fr.RequesterID == userID {
			other = fr.AddresseeID
		} else {
			other = fr.RequesterID
		}
		if _, ok := seen[other]; ok {
			continue
		}
		seen[other] = struct{}{}
		ids = append(ids, other)
	}

	return ids, nil
}

func followedIDsFor(db *gorm.DB, userID uint) ([]uint, error) {
	var follows []models.Follow
	if err := db.Where("follower_id = ?", userID).Find(&follows).Error; err != nil {
		return nil, err
	}

	ids := make([]uint, 0, len(follows))
	seen := map[uint]struct{}{}
	for _, f := range follows {
		if _, ok := seen[f.FolloweeID]; ok {
			continue
		}
		seen[f.FolloweeID] = struct{}{}
		ids = append(ids, f.FolloweeID)
	}

	return ids, nil
}

func loadUsersByIDs(db *gorm.DB, ids []uint) ([]models.User, error) {
	if len(ids) == 0 {
		return []models.User{}, nil
	}

	var users []models.User
	if err := db.Where("id IN ?", ids).Find(&users).Error; err != nil {
		return nil, err
	}

	return users, nil
}

func createNotification(db *gorm.DB, userID uint, title, content, style string) error {
	if style == "" {
		style = "default"
	}

	n := models.Notification{
		UserID:  userID,
		Title:   title,
		Content: content,
		Style:   style,
	}

	return db.Create(&n).Error
}
