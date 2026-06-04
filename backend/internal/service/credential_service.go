package service

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/warden-stock/warden-stock-data/internal/model"
	"github.com/warden-stock/warden-stock-data/internal/repository"
	"github.com/warden-stock/warden-stock-data/pkg/crypto"
	"github.com/warden-stock/warden-stock-data/pkg/errcode"
	"github.com/warden-stock/warden-stock-data/pkg/utils"
)

type CredentialService struct {
	repo   *repository.CredentialRepository
	encKey []byte
}

type CredentialSecret struct {
	ID        uint   `json:"id"`
	SecretID  string `json:"secret_id"`
	SecretKey string `json:"secret_key"`
}

func NewCredentialService(repo *repository.CredentialRepository, encKey string) *CredentialService {
	return &CredentialService{repo: repo, encKey: crypto.NormalizeEncKey(encKey)}
}

func (s *CredentialService) Create(ctx context.Context, adminID uint, consumerName string, rateLimit, dailyQuota int, expireAt *time.Time) (*CredentialSecret, error) {
	secretID, err := utils.NewSecretID()
	if err != nil {
		return nil, err
	}
	secretKey, err := utils.NewSecretKey()
	if err != nil {
		return nil, err
	}
	cipher, err := crypto.EncryptAESGCM(s.encKey, secretKey)
	if err != nil {
		return nil, err
	}
	hash, err := crypto.HashSecretKey(secretKey)
	if err != nil {
		return nil, err
	}
	if rateLimit <= 0 {
		rateLimit = 20
	}
	if dailyQuota <= 0 {
		dailyQuota = 100000
	}
	cred := &model.APICredential{
		SecretID: secretID, SecretKeyCipher: cipher, SecretKeyHash: hash,
		ConsumerName: consumerName, Scope: "read",
		RateLimit: rateLimit, DailyQuota: dailyQuota, Status: 1,
		ExpireAt: expireAt, CreatedBy: adminID,
	}
	if err := s.repo.Create(ctx, cred); err != nil {
		return nil, err
	}
	return &CredentialSecret{ID: cred.ID, SecretID: secretID, SecretKey: secretKey}, nil
}

func (s *CredentialService) List(ctx context.Context, page, size int) ([]model.APICredential, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	return s.repo.List(ctx, page, size)
}

func (s *CredentialService) Get(ctx context.Context, id uint) (*model.APICredential, error) {
	cred, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcodeBiz{code: errcode.ErrNotFound}
		}
		return nil, err
	}
	return cred, nil
}

func (s *CredentialService) Update(ctx context.Context, id uint, rateLimit, dailyQuota *int, status *int16, expireAt *time.Time) error {
	cred, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if rateLimit != nil {
		cred.RateLimit = *rateLimit
	}
	if dailyQuota != nil {
		cred.DailyQuota = *dailyQuota
	}
	if status != nil {
		cred.Status = *status
	}
	if expireAt != nil {
		cred.ExpireAt = expireAt
	}
	return s.repo.Update(ctx, cred)
}

func (s *CredentialService) Revoke(ctx context.Context, id uint) error {
	if _, err := s.Get(ctx, id); err != nil {
		return err
	}
	return s.repo.Revoke(ctx, id)
}

func (s *CredentialService) Rotate(ctx context.Context, id uint) (*CredentialSecret, error) {
	cred, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	secretKey, err := utils.NewSecretKey()
	if err != nil {
		return nil, err
	}
	cipher, err := crypto.EncryptAESGCM(s.encKey, secretKey)
	if err != nil {
		return nil, err
	}
	hash, err := crypto.HashSecretKey(secretKey)
	if err != nil {
		return nil, err
	}
	cred.SecretKeyCipher = cipher
	cred.SecretKeyHash = hash
	if err := s.repo.Update(ctx, cred); err != nil {
		return nil, err
	}
	return &CredentialSecret{ID: cred.ID, SecretID: cred.SecretID, SecretKey: secretKey}, nil
}

func (s *CredentialService) ResolveSecretKey(ctx context.Context, secretID string) (string, *model.APICredential, error) {
	cred, err := s.repo.FindBySecretID(ctx, secretID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil, errcodeBiz{code: errcode.ErrCredentialInvalid}
		}
		return "", nil, err
	}
	key, err := crypto.DecryptAESGCM(s.encKey, cred.SecretKeyCipher)
	if err != nil {
		return "", nil, errcodeBiz{code: errcode.ErrCredentialInvalid}
	}
	return key, cred, nil
}
