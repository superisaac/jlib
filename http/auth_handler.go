package jsonzhttp

import (
	"context"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/hashicorp/golang-lru"
	"github.com/pkg/errors"
	"net/http"
	"strings"
	"time"
)

type BasicAuthConfig struct {
	Username string
	Password string
}

type BearerAuthConfig struct {
	Token string

	// username attached to request when token authorized
	Username string `yaml:"username,omitempty" json:"username,omitempty"`
}

type JwtAuthConfig struct {
	Secret string
}

type AuthConfig struct {
	Basic  []BasicAuthConfig  `yaml:"basic,omitempty" json:"basic,omitempty"`
	Bearer []BearerAuthConfig `yaml:"bearer,omitempty" json:"bearer,omitempty"`
	Jwt    *JwtAuthConfig     `yaml:"jwt,omitempty" json:"jwt,omitempty"`
}

type jwtClaims struct {
	Username string
	jwt.StandardClaims
}

// Auth handler
type HttpAuthHandler struct {
	authConfig *AuthConfig
	next       http.Handler
	jwtCache   *lru.Cache
}

func NewHttpAuthHandler(authConfig *AuthConfig, next http.Handler) *HttpAuthHandler {
	cache, err := lru.New(100)
	if err != nil {
		panic(err)
	}
	return &HttpAuthHandler{
		authConfig: authConfig,
		jwtCache:   cache,
		next:       next,
	}
}

func (self HttpAuthHandler) TryAuth(r *http.Request) (string, bool) {
	if self.authConfig == nil {
		return "", true
	}

	if self.authConfig.Jwt != nil && self.authConfig.Jwt.Secret != "" {
		if username, ok := self.jwtAuth(self.authConfig.Jwt, r); ok {
			return username, true
		}
	}

	if self.authConfig.Basic != nil && len(self.authConfig.Basic) > 0 {
		if username, password, ok := r.BasicAuth(); ok {
			for _, basicCfg := range self.authConfig.Basic {
				if basicCfg.Username == username && basicCfg.Password == password {
					return username, true
				}
			}
		}
	}

	if self.authConfig.Bearer != nil && len(self.authConfig.Bearer) > 0 {
		authHeader := r.Header.Get("Authorization")
		for _, bearCfg := range self.authConfig.Bearer {
			expect := fmt.Sprintf("Bearer %s", bearCfg.Token)
			if authHeader == expect {
				username := bearCfg.Username
				if username == "" {
					username = bearCfg.Token
				}
				return username, true
			}
		}
	}

	return "", false
}

func (self *HttpAuthHandler) jwtAuth(jwtCfg *JwtAuthConfig, r *http.Request) (string, bool) {
	// refers to https://qvault.io/cryptography/jwts-in-golang/
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", false
	}
	if arr := strings.SplitN(authHeader, " ", 2); len(arr) <= 2 && arr[0] == "Bearer" {
		var claims *jwtClaims
		fromCache := false

		if cached, ok := self.jwtCache.Get(authHeader); ok {
			claims, _ = cached.(*jwtClaims)
			fromCache = true
		} else {
			jwtFromHeader := arr[1]
			token, err := jwt.ParseWithClaims(
				jwtFromHeader,
				&jwtClaims{},
				func(token *jwt.Token) (interface{}, error) {
					return []byte(jwtCfg.Secret), nil
				},
			)
			if err != nil {
				Logger(r).Warnf("jwt auth error %s", err)
				return "", false
			}
			claims, ok = token.Claims.(*jwtClaims)
			if !ok {
				return "", false
			}
		}
		// check expiration
		if claims.ExpiresAt < time.Now().UTC().Unix() {
			Logger(r).Warnf("claims expired %s", authHeader)
			if fromCache {
				self.jwtCache.Remove(authHeader)
			}
			return "", false
		}
		if !fromCache {
			self.jwtCache.Add(authHeader, claims)
		}
		return claims.Username, true
	}
	return "", false
}

func (self *HttpAuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if username, ok := self.TryAuth(r); ok {
		ctx := context.WithValue(r.Context(), "username", username)
		self.next.ServeHTTP(w, r.WithContext(ctx))
	} else {
		w.WriteHeader(401)
		w.Write([]byte("auth failed!\n"))
	}
}

// Auth config
func (self *AuthConfig) ValidateValues() error {
	if self == nil {
		return nil
	}

	if self.Bearer != nil && len(self.Bearer) > 0 {
		for _, bearCfg := range self.Bearer {
			if bearCfg.Token == "" {
				return errors.New("bearer token cannot be empty")
			}
		}
	}

	if self.Basic != nil && len(self.Basic) > 0 {
		for _, basicCfg := range self.Basic {
			if basicCfg.Username == "" || basicCfg.Password == "" {
				return errors.New("basic username and password cannot be empty")
			}
		}
	}

	if self.Jwt != nil && self.Jwt.Secret != "" {
		return errors.New("jwt has no secret")
	}
	return nil
}
