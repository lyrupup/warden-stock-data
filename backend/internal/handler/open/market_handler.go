package open

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/warden-stock/warden-stock-data/internal/service"
	"github.com/warden-stock/warden-stock-data/pkg/errcode"
	"github.com/warden-stock/warden-stock-data/pkg/response"
	"github.com/warden-stock/warden-stock-data/pkg/utils"
)

type MarketHandler struct {
	quote     *service.QuoteService
	kline     *service.KlineService
	indicator *service.IndicatorService
	meta      *service.MetaService
}

func NewMarketHandler(
	quote *service.QuoteService,
	kline *service.KlineService,
	indicator *service.IndicatorService,
	meta *service.MetaService,
) *MarketHandler {
	return &MarketHandler{quote: quote, kline: kline, indicator: indicator, meta: meta}
}

func (h *MarketHandler) Indices(c *gin.Context) {
	list, err := h.quote.Indices(c.Request.Context(), c.DefaultQuery("market", "CN"))
	if err != nil {
		response.Fail(c, http.StatusOK, errcode.ErrProvider)
		return
	}
	response.OK(c, list)
}

func (h *MarketHandler) Quotes(c *gin.Context) {
	codes := utils.SplitCSV(c.Query("codes"))
	list, err := h.quote.Quotes(c.Request.Context(), codes)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, service.BizCode(err))
		return
	}
	response.OK(c, list)
}

func (h *MarketHandler) Stock(c *gin.Context) {
	q, err := h.quote.Quote(c.Request.Context(), c.Param("code"))
	if err != nil {
		response.Fail(c, http.StatusNotFound, service.BizCode(err))
		return
	}
	response.OK(c, q)
}

func (h *MarketHandler) Kline(c *gin.Context) {
	q := service.KlineQuery{
		Code:   c.Param("code"),
		Period: c.DefaultQuery("period", "day"),
		Adjust: c.DefaultQuery("adjust", "qfq"),
		Limit:  120,
	}
	if limitStr := c.Query("limit"); limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil {
			q.Limit = n
		}
	}
	if from := c.Query("from"); from != "" {
		if t, err := time.Parse("2006-01-02", from); err == nil {
			q.From = &t
			q.Limit = 0
		}
	}
	if to := c.Query("to"); to != "" {
		if t, err := time.Parse("2006-01-02", to); err == nil {
			q.To = &t
			q.Limit = 0
		}
	}
	bars, err := h.kline.Kline(c.Request.Context(), q)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, service.BizCode(err))
		return
	}
	response.OK(c, bars)
}

func (h *MarketHandler) Intraday(c *gin.Context) {
	res, err := h.quote.Intraday(c.Request.Context(), c.Param("code"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, service.BizCode(err))
		return
	}
	response.OK(c, res)
}

func (h *MarketHandler) StockIndicators(c *gin.Context) {
	types := utils.SplitCSV(c.DefaultQuery("types", "ma5,ma10,ma20,ma30,ma60"))
	res, err := h.indicator.ComputeForStock(c.Request.Context(), c.Param("code"), types)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, service.BizCode(err))
		return
	}
	response.OK(c, res)
}

func (h *MarketHandler) BatchIndicators(c *gin.Context) {
	codes := utils.SplitCSV(c.Query("codes"))
	types := utils.SplitCSV(c.DefaultQuery("types", "ma5,ma10,ma20,ma30,ma60"))
	var tradeDate *time.Time
	if td := c.Query("trade_date"); td != "" {
		if t, err := time.Parse("2006-01-02", td); err == nil {
			tradeDate = &t
		}
	}
	list, err := h.indicator.BatchIndicators(c.Request.Context(), codes, types, tradeDate)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, errcode.ErrInternal)
		return
	}
	response.OK(c, list)
}

func (h *MarketHandler) Search(c *gin.Context) {
	list, err := h.quote.Search(c.Request.Context(), c.Query("kw"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, service.BizCode(err))
		return
	}
	response.OK(c, list)
}

func (h *MarketHandler) Securities(c *gin.Context) {
	list, err := h.quote.Securities(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, errcode.ErrInternal)
		return
	}
	response.OK(c, list)
}

func (h *MarketHandler) Meta(c *gin.Context) {
	m, err := h.meta.Meta(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, errcode.ErrInternal)
		return
	}
	response.OK(c, m)
}
