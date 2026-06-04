package admin

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/warden-stock/warden-stock-data/internal/service"
	"github.com/warden-stock/warden-stock-data/pkg/errcode"
	"github.com/warden-stock/warden-stock-data/pkg/response"
)

type AuthHandler struct {
	adminSvc *service.AdminService
}

func NewAuthHandler(adminSvc *service.AdminService) *AuthHandler {
	return &AuthHandler{adminSvc: adminSvc}
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, errcode.ErrParam)
		return
	}
	token, err := h.adminSvc.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, service.BizCode(err))
		return
	}
	response.OK(c, gin.H{"token": token})
}

func (h *AuthHandler) Me(c *gin.Context) {
	adminID := c.GetUint("admin_id")
	admin, err := h.adminSvc.Me(c.Request.Context(), adminID)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, service.BizCode(err))
		return
	}
	response.OK(c, gin.H{
		"id":       admin.ID,
		"username": admin.Username,
		"role":     admin.Role,
	})
}
