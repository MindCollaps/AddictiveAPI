package ws

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"addictiveapi/internal/auth"
)

const claimsContextKey = "wsClaims"

func JWTMiddleware(authService *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Try Authorization header first (Bearer <token>)
		tokenString, ok := extractBearerToken(c.GetHeader("Authorization"))

		// If missing, allow token as query parameter for browser clients: /ws?token=<jwt>
		if !ok {
			if q := c.Query("token"); q != "" {
				tokenString = q
				ok = true
			}
		}

		// Also accept token in Sec-Websocket-Protocol (some ws clients use this)
		if !ok {
			if proto := c.GetHeader("Sec-Websocket-Protocol"); proto != "" {
				parts := strings.Split(proto, ",")
				for i := range parts {
					parts[i] = strings.TrimSpace(parts[i])
				}

				// If first part == "token" and second exists, use second as jwt and echo "token"
				if len(parts) >= 2 && strings.EqualFold(parts[0], "token") {
					tokenString = parts[1]
					if tokenString != "" {
						ok = true
						c.Set("wsSubprotocol", "token")
					}
				} else {
					// check for token=<jwt> form in any part
					for _, p := range parts {
						if strings.HasPrefix(strings.ToLower(p), "token=") {
							tokenString = strings.TrimSpace(p[len("token="):])
							if tokenString != "" {
								ok = true
								c.Set("wsSubprotocol", p[:strings.Index(p, "=")])
							}
							break
						}
					}
				}
			}
		}

		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}

		claims, err := authService.ParseToken(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		expiresAt, err := claims.GetExpirationTime()
		if err != nil || expiresAt == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token expiration unavailable"})
			return
		}

		c.Set(claimsContextKey, claims)
		c.Set("wsToken", tokenString)
		c.Set("wsTokenExpiresAt", expiresAt.Time)
		c.Next()
	}
}

func extractBearerToken(header string) (string, bool) {
	parts := strings.Fields(header)
	if len(parts) != 2 {
		return "", false
	}

	if !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}

	return parts[1], true
}
