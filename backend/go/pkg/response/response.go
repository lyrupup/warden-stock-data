package response

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/warden-stock/warden-stock-data/pkg/errcode"
)

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type PageData struct {
	List  interface{} `json:"list"`
	Total int64       `json:"total"`
	Page  int         `json:"page"`
	Size  int         `json:"size"`
}

func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    errcode.OK,
		Message: errcode.Message(errcode.OK),
		Data:    data,
	})
}

func Page(c *gin.Context, list interface{}, total int64, page, size int) {
	OK(c, PageData{
		List:  list,
		Total: total,
		Page:  page,
		Size:  size,
	})
}

func Fail(c *gin.Context, httpStatus, code int) {
	c.JSON(httpStatus, Response{
		Code:    code,
		Message: errcode.Message(code),
	})
}

func FailWithMessage(c *gin.Context, httpStatus, code int, message string) {
	c.JSON(httpStatus, Response{
		Code:    code,
		Message: message,
	})
}
