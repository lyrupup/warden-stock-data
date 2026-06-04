package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/warden-stock/warden-stock-data/internal/model"
)

type AdminRepository struct {
	db *gorm.DB
}

func NewAdminRepository(db *gorm.DB) *AdminRepository {
	return &AdminRepository{db: db}
}

func (r *AdminRepository) FindByUsername(ctx context.Context, username string) (*model.Admin, error) {
	var admin model.Admin
	err := r.db.WithContext(ctx).Where("username = ? AND status = 1", username).First(&admin).Error
	if err != nil {
		return nil, err
	}
	return &admin, nil
}

func (r *AdminRepository) FindByID(ctx context.Context, id uint) (*model.Admin, error) {
	var admin model.Admin
	err := r.db.WithContext(ctx).First(&admin, id).Error
	if err != nil {
		return nil, err
	}
	return &admin, nil
}

func (r *AdminRepository) EnsureDefault(ctx context.Context, username, passwordHash string) error {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.Admin{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&model.Admin{
		Username:     username,
		PasswordHash: passwordHash,
		Role:         "admin",
		Status:       1,
	}).Error
}
