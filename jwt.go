package main

import (
	"context"
	"encoding/base64"
	"strings"
	"time"

	errs "errors"

	"github.com/dgrijalva/jwt-go"
	"gopkg.in/oauth2.v3"
	"gopkg.in/oauth2.v3/errors"
	"gopkg.in/oauth2.v3/utils/uuid"
)

type JWTUser struct {
	Email string `json:"email"`
}

// JWTAccessClaims jwt claims
type JWTAccessClaims struct {
	Scope string  `json:"scope"`
	User  JWTUser `json:"user"`
	jwt.StandardClaims
}

// Valid claims verification
func (a *JWTAccessClaims) Valid() error {
	if time.Unix(a.ExpiresAt, 0).Before(time.Now()) {
		return errors.ErrInvalidAccessToken
	}
	return nil
}

// NewJWTAccessGenerate create to generate the jwt access token instance
func NewJWTAccessGenerate(key interface{}, method jwt.SigningMethod, userStore *UserStore) *JWTAccessGenerate {
	return &JWTAccessGenerate{
		SignedKey:    key,
		SignedMethod: method,
		UserStore:    userStore,
	}
}

// JWTAccessGenerate generate the jwt access token
type JWTAccessGenerate struct {
	SignedKey    interface{}
	SignedMethod jwt.SigningMethod
	UserStore    *UserStore
}

// Token based on the UUID generated token
func (a *JWTAccessGenerate) Token(data *oauth2.GenerateBasic, isGenRefresh bool) (access, refresh string, err error) {
	ctx := context.Background()
	scope := data.Request.FormValue("scope")

	user, fetchErr := a.UserStore.GetUser(data.UserID)
	err = fetchErr
	if err != nil {
		return
	}

	if scope != "" {
		scope, err = validateScopeForUser(ctx, scope, data.UserID)
		if err != nil {
			return
		}
	}

	claims := &JWTAccessClaims{
		StandardClaims: jwt.StandardClaims{
			Audience:  data.Client.GetID(),
			Subject:   data.UserID,
			ExpiresAt: data.TokenInfo.GetAccessCreateAt().Add(data.TokenInfo.GetAccessExpiresIn()).Unix(),
		},
		Scope: scope,
		User: JWTUser{
			Email: user.Email,
		},
	}

	token := jwt.NewWithClaims(a.SignedMethod, claims)
	var key interface{}
	if a.isEs() {
		key, err = jwt.ParseECPrivateKeyFromPEM(a.SignedKey.([]byte))
		if err != nil {
			return "", "", err
		}
	} else if a.isRsOrPS() {
		key = a.SignedKey
	} else if a.isHs() {
		key = a.SignedKey
	} else {
		return "", "", errs.New("unsupported sign method")
	}
	access, err = token.SignedString(key)
	if err != nil {
		return
	}

	if isGenRefresh {
		refresh = base64.URLEncoding.EncodeToString(uuid.NewSHA1(uuid.Must(uuid.NewRandom()), []byte(access)).Bytes())
		refresh = strings.ToUpper(strings.TrimRight(refresh, "="))
	}

	return
}

func (a *JWTAccessGenerate) isEs() bool {
	return strings.HasPrefix(a.SignedMethod.Alg(), "ES")
}

func (a *JWTAccessGenerate) isRsOrPS() bool {
	isRs := strings.HasPrefix(a.SignedMethod.Alg(), "RS")
	isPs := strings.HasPrefix(a.SignedMethod.Alg(), "PS")
	return isRs || isPs
}

func (a *JWTAccessGenerate) isHs() bool {
	return strings.HasPrefix(a.SignedMethod.Alg(), "HS")
}
