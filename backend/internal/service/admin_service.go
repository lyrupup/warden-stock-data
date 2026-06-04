package service

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"

	"github.com/warden-stock/warden-stock-data/internal/model"
	"github.com/warden-stock/warden-stock-data/internal/repository"
	"github.com/warden-stock/warden-stock-data/pkg/crypto"
	"github.com/warden-stock/warden-stock-data/pkg/errcode"
)

type AdminService struct {
	repo      *repository.AdminRepository
	jwtSecret []byte
	jwtExpire time.Duration
}

func NewAdminService(repo *repository.AdminRepository, jwtSecret string, expire time.Duration) *AdminService {
	return &AdminService{repo: repo, jwtSecret: []byte(jwtSecret), jwtExpire: expire}
}

type adminClaims struct {
	AdminID uint `json:"admin_id"`
	jwt.RegisteredClaims
}

func (s *AdminService) Login(ctx context.Context, username, password string) (string, error) {
	admin, err := s.repo.FindByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", errcodeBiz{code: errcode.ErrAdminUnauthorized}
		}
		return "", err
	}
	if !crypto.CheckPassword(admin.PasswordHash, password) {
		return "", errcodeBiz{code: errcode.ErrAdminUnauthorized}
	}
	now := time.Now()
	claims := adminClaims{
		AdminID: admin.ID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(s.jwtExpire)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

func (s *AdminService) ParseToken(tokenStr string) (uint, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &adminClaims{}, func(t *jwt.Token) (interface{}, error) {
		return s.jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return 0, errcodeBiz{code: errcode.ErrAdminUnauthorized}
	}
	claims, ok := token.Claims.(*adminClaims)
	if !ok {
		return 0, errcodeBiz{code: errcode.ErrAdminUnauthorized}
	}
	return claims.AdminID, nil
}

func (s *AdminService) Me(ctx context.Context, adminID uint) (*model.Admin, error) {
	return s.repo.FindByID(ctx, adminID)
}

type errcodeBiz struct{ code int }

func (e errcodeBiz) Error() string { return errcode.Message(e.code) }

func (e errcodeBiz) Code() int { return e.code }

func BizCode(err error) int {
	var b errcodeBiz
	if errors.As(err, &b) {
		return b.Code()
	}
	return errcode.ErrInternal
}
