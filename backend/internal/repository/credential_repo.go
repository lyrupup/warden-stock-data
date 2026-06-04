package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/warden-stock/warden-stock-data/internal/model"
)

type CredentialRepository struct {
	db *gorm.DB
}

func NewCredentialRepository(db *gorm.DB) *CredentialRepository {
	return &CredentialRepository{db: db}
}

func (r *CredentialRepository) Create(ctx context.Context, cred *model.APICredential) error {
	return r.db.WithContext(ctx).Create(cred).Error
}

func (r *CredentialRepository) FindBySecretID(ctx context.Context, secretID string) (*model.APICredential, error) {
	var cred model.APICredential
	err := r.db.WithContext(ctx).Where("secret_id = ? AND status = 1", secretID).First(&cred).Error
	if err != nil {
		return nil, err
	}
	if cred.ExpireAt != nil && cred.ExpireAt.Before(time.Now()) {
		return nil, gorm.ErrRecordNotFound
	}
	return &cred, nil
}

func (r *CredentialRepository) List(ctx context.Context, page, size int) ([]model.APICredential, int64, error) {
	var list []model.APICredential
	var total int64
	q := r.db.WithContext(ctx).Model(&model.APICredential{})
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * size
	err := q.Order("id desc").Offset(offset).Limit(size).Find(&list).Error
	return list, total, err
}

func (r *CredentialRepository) FindByID(ctx context.Context, id uint) (*model.APICredential, error) {
	var cred model.APICredential
	err := r.db.WithContext(ctx).First(&cred, id).Error
	if err != nil {
		return nil, err
	}
	return &cred, nil
}

func (r *CredentialRepository) Update(ctx context.Context, cred *model.APICredential) error {
	return r.db.WithContext(ctx).Save(cred).Error
}

func (r *CredentialRepository) Revoke(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&model.APICredential{}).Where("id = ?", id).Update("status", 0).Error
}
