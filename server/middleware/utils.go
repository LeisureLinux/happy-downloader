package middlewares

import (
	"net/http"

	"github.com/LeisureLinux/happy-downloader/v4/server/config"
	"github.com/LeisureLinux/happy-downloader/v4/server/openid"
)

func ApplyAuthenticationByConfig(next http.Handler) http.Handler {
	handler := next

	if config.Instance().Authentication.RequireAuth {
		handler = Authenticated(handler)
	}
	if config.Instance().OpenId.UseOpenId {
		handler = openid.Middleware(handler)
	}

	return handler
}
