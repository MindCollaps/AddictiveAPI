package ws

import "time"

type HandlerFunc func(*Context, Message) Response

type Registry struct {
	handlers map[string]map[string]HandlerFunc
}

func NewRegistry() *Registry {
	return &Registry{handlers: make(map[string]map[string]HandlerFunc)}
}

func (r *Registry) Register(topic, command string, handler HandlerFunc) {
	if r.handlers[topic] == nil {
		r.handlers[topic] = make(map[string]HandlerFunc)
	}

	r.handlers[topic][command] = handler
}

func (r *Registry) Dispatch(ctx *Context, message Message) Response {
	if commands := r.handlers[message.Topic]; commands != nil {
		if handler := commands[message.Command]; handler != nil {
			return handler(ctx, message)
		}
	}

	return Response{
		Topic:   message.Topic,
		Command: message.Command,
		Status:  "accepted",
		Data: map[string]any{
			"message":   "handler not implemented yet",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		},
	}
}
