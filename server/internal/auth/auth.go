// Package auth provides session-cookie authentication and role-gating
// middleware on top of the store package's user/session tables.
package auth

import (
	"context"
	"net/http"
	"time"

	"serverdash/internal/store"
)

const CookieName = "serverdash_session"

type contextKey int

const userContextKey contextKey = 0

// SetSessionCookie issues the session cookie for a freshly created session.
// Secure is left true always: ServerDash is expected to sit behind a TLS
// reverse proxy (nova.blacklink.net); a plain-HTTP deployment would need
// this relaxed, but shipping it secure-by-default is the safer choice.
func SetSessionCookie(w http.ResponseWriter, sess store.Session) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    sess.Token,
		Path:     "/",
		Expires:  sess.ExpiresAt,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

func UserFromContext(ctx context.Context) (store.User, bool) {
	u, ok := ctx.Value(userContextKey).(store.User)
	return u, ok
}

// Middleware authenticates every request against the session cookie and,
// on success, injects the user into the request context. It's applied to
// the whole API except the handful of pre-auth routes (setup status,
// login) that register themselves before it via a path allowlist.
type Middleware struct {
	Store       *store.Store
	PublicPaths map[string]bool
}

func (m *Middleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.PublicPaths[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}

		cookie, err := r.Cookie(CookieName)
		if err != nil {
			http.Error(w, `{"error":"authentication required"}`, http.StatusUnauthorized)
			return
		}

		user, err := m.Store.UserBySession(r.Context(), cookie.Value)
		if err != nil {
			http.Error(w, `{"error":"authentication required"}`, http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireRole wraps a handler so it 403s unless the authenticated user
// (already injected by Middleware) has one of the allowed roles.
func RequireRole(next http.HandlerFunc, allowed ...store.Role) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := UserFromContext(r.Context())
		if !ok {
			http.Error(w, `{"error":"authentication required"}`, http.StatusUnauthorized)
			return
		}
		for _, role := range allowed {
			if user.Role == role {
				next.ServeHTTP(w, r)
				return
			}
		}
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
	}
}
