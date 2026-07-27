package httptransport

import (
	"log/slog"
	"net/http"

	"github.com/hiendangba/MoneyTrackingBE/Backend/auth-service/internal/dto"
	apperrors "github.com/hiendangba/MoneyTrackingBE/Backend/auth-service/internal/errors"
	"github.com/hiendangba/MoneyTrackingBE/Backend/auth-service/internal/service"
)

type MenuHandler struct {
	menuService *service.MenuService
	logger      *slog.Logger
}

func NewMenuHandler(menuService *service.MenuService, logger *slog.Logger) *MenuHandler {
	return &MenuHandler{menuService: menuService, logger: logger}
}

func (h *MenuHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateMenuRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, apperrors.Validation("invalid request body"), h.logger)
		return
	}
	response, err := h.menuService.Create(r.Context(), req)
	if err != nil {
		writeError(w, err, h.logger)
		return
	}
	writeJSON(w, http.StatusCreated, response)
}

func (h *MenuHandler) List(w http.ResponseWriter, r *http.Request) {
	response, err := h.menuService.List(r.Context())
	if err != nil {
		writeError(w, err, h.logger)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *MenuHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	response, err := h.menuService.GetByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err, h.logger)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *MenuHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateMenuRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, apperrors.Validation("invalid request body"), h.logger)
		return
	}
	response, err := h.menuService.Update(r.Context(), r.PathValue("id"), req)
	if err != nil {
		writeError(w, err, h.logger)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *MenuHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.menuService.Delete(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, err, h.logger)
		return
	}
	writeJSON(w, http.StatusOK, dto.MessageResponse{Message: "menu deleted"})
}

func (h *MenuHandler) Tree(w http.ResponseWriter, r *http.Request) {
	response, err := h.menuService.Tree(r.Context())
	if err != nil {
		writeError(w, err, h.logger)
		return
	}
	writeJSON(w, http.StatusOK, response)
}
