// Copyright 2020 Northern.tech AS
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//        http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

package identity

import (
	"net/http"
	"regexp"

	"github.com/ant0ine/go-json-rest/rest"
	"github.com/gin-gonic/gin"

	"github.com/mendersoftware/go-lib-micro/log"
	urest "github.com/mendersoftware/go-lib-micro/rest.utils"
)

const (
	// IdentityPathsRe decides which requests
	defaultPathRegex = "^/api/management/v[0-9.]+/.+"
)

type MiddlewareOptions struct {
	// PathRegex sets the regex for the path for which this middleware
	// applies. Defaults to "^/api/management/v[0-9.]{1,6}/.+".
	PathRegex *string

	// UpdateLogger adds the decoded identity to the log context.
	UpdateLogger *bool
}

func NewMiddlewareOptions() *MiddlewareOptions {
	return new(MiddlewareOptions)
}

func (opts *MiddlewareOptions) SetPathRegex(regex string) *MiddlewareOptions {
	opts.PathRegex = &regex
	return opts
}

func (opts *MiddlewareOptions) SetUpdateLogger(updateLogger bool) *MiddlewareOptions {
	opts.UpdateLogger = &updateLogger
	return opts
}

func Middleware(opts ...*MiddlewareOptions) gin.HandlerFunc {
	// Initialize default options
	opt := NewMiddlewareOptions().
		SetPathRegex(defaultPathRegex).
		SetUpdateLogger(true)
	for _, o := range opts {
		if o == nil {
			continue
		}
		if o.PathRegex != nil {
			opt.PathRegex = o.PathRegex
		}
		if o.UpdateLogger != nil {
			opt.UpdateLogger = o.UpdateLogger
		}
	}
	pathRegex := regexp.MustCompile(*opt.PathRegex)

	return func(c *gin.Context) {
		if !pathRegex.MatchString(c.FullPath()) {
			return
		}

		var (
			err    error
			jwt    string
			idty   Identity
			logCtx = log.Ctx{}
			key    = "sub"
			ctx    = c.Request.Context()
			l      = log.FromContext(ctx)
		)
		jwt, err = ExtractJWTFromHeader(c.Request)
		if err != nil {
			goto exitUnauthorized
		}
		idty, err = ExtractIdentity(jwt)
		if err != nil {
			goto exitUnauthorized
		}
		ctx = WithContext(ctx, &idty)
		if *opt.UpdateLogger {
			if idty.IsDevice {
				key = "device_id"
			} else if idty.IsUser {
				key = "user_id"
			}
			logCtx[key] = idty.Subject
			if idty.Tenant != "" {
				logCtx["tenant_id"] = idty.Tenant
			}
			if idty.Plan != "" {
				logCtx["plan"] = idty.Plan
			}
			ctx = log.WithContext(ctx, l.F(logCtx))
		}

		c.Request = c.Request.WithContext(ctx)
		return
	exitUnauthorized:
		c.Header("WWW-Authenticate", `Bearer realm="ManagementJWT"`)
		urest.RenderError(c, http.StatusUnauthorized, err)
		c.Abort()
	}
}

// IdentityMiddleware adds the identity extracted from JWT token to the request's context.
// IdentityMiddleware does not perform any form of token signature verification.
// If it is not possible to extract identity from header error log will be generated.
// IdentityMiddleware will not stop control propagating through the chain in any case.
// It is recommended to use IdentityMiddleware with RequestLogMiddleware and
// RequestLogMiddleware should be placed before IdentityMiddleware.
// Otherwise, log generated by IdentityMiddleware will not contain "request_id" field.
type IdentityMiddleware struct {
	// If set to true, the middleware will update context logger setting
	// 'user_id' or 'device_id' to the value of subject field, if the token
	// is not a user or a device token, the middelware will add a 'sub'
	// field to the logger
	UpdateLogger bool
}

// MiddlewareFunc makes IdentityMiddleware implement the Middleware interface.
func (mw *IdentityMiddleware) MiddlewareFunc(h rest.HandlerFunc) rest.HandlerFunc {
	return func(w rest.ResponseWriter, r *rest.Request) {
		jwt, err := ExtractJWTFromHeader(r.Request)
		if err != nil {
			h(w, r)
			return
		}

		ctx := r.Context()
		l := log.FromContext(ctx)

		identity, err := ExtractIdentity(jwt)
		if err != nil {
			l.Warnf("Failed to parse extracted JWT: %s",
				err.Error(),
			)
		} else {
			if mw.UpdateLogger {
				logCtx := log.Ctx{}

				key := "sub"
				if identity.IsDevice {
					key = "device_id"
				} else if identity.IsUser {
					key = "user_id"
				}

				logCtx[key] = identity.Subject

				if identity.Tenant != "" {
					logCtx["tenant_id"] = identity.Tenant
				}

				if identity.Plan != "" {
					logCtx["plan"] = identity.Plan
				}

				l = l.F(logCtx)
				ctx = log.WithContext(ctx, l)
			}
			ctx = WithContext(ctx, &identity)
			r.Request = r.WithContext(ctx)
		}

		h(w, r)
	}
}
