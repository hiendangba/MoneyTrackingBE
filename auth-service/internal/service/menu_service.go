package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"auth-service/internal/domain"
	"auth-service/internal/dto"
	apperrors "auth-service/internal/errors"
	"auth-service/internal/repository"
	"auth-service/internal/utils"

	"github.com/google/uuid"
)

type MenuService struct {
	menuRepo repository.MenuRepository
}

type menuTreeNode struct {
	response  dto.MenuTreeResponse
	sortOrder int
	children  []*menuTreeNode
}

func NewMenuService(menuRepo repository.MenuRepository) *MenuService {
	return &MenuService{menuRepo: menuRepo}
}

func (s *MenuService) Create(ctx context.Context, req dto.CreateMenuRequest) (*dto.MenuResponse, error) {
	menu, err := s.buildCreateMenu(ctx, req)
	if err != nil {
		return nil, err
	}

	created, err := s.menuRepo.Create(ctx, menu)
	if err != nil {
		if err == apperrors.ErrConflict {
			return nil, mapMenuConflict(ctx, s.menuRepo, menu.ID, menu.Code, menu.Route)
		}
		return nil, err
	}
	return toMenuResponse(created), nil
}

func (s *MenuService) List(ctx context.Context) ([]dto.MenuResponse, error) {
	menus, err := s.menuRepo.List(ctx)
	if err != nil {
		return nil, err
	}

	responses := make([]dto.MenuResponse, 0, len(menus))
	for _, menu := range menus {
		responses = append(responses, *toMenuResponse(&menu))
	}
	return responses, nil
}

func (s *MenuService) GetByID(ctx context.Context, id string) (*dto.MenuResponse, error) {
	menu, err := s.menuRepo.FindByID(ctx, strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}
	return toMenuResponse(menu), nil
}

func (s *MenuService) Update(ctx context.Context, id string, req dto.UpdateMenuRequest) (*dto.MenuResponse, error) {
	menu, err := s.menuRepo.FindByID(ctx, strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}

	if req.Code != nil {
		menu.Code = strings.TrimSpace(*req.Code)
	}
	if req.Name != nil {
		menu.Name = strings.TrimSpace(*req.Name)
	}
	if req.Route != nil {
		menu.Route = strings.TrimSpace(*req.Route)
	}
	if req.Icon != nil {
		icon := strings.TrimSpace(*req.Icon)
		if icon == "" {
			menu.Icon = nil
		} else {
			menu.Icon = &icon
		}
	}
	if req.ParentID != nil {
		parentID := strings.TrimSpace(*req.ParentID)
		if parentID == "" {
			menu.ParentID = nil
		} else {
			menu.ParentID = &parentID
		}
	}
	if req.SortOrder != nil {
		menu.SortOrder = *req.SortOrder
	}
	if req.IsActive != nil {
		menu.IsActive = *req.IsActive
	}

	if err := s.validateMenu(ctx, menu, false); err != nil {
		return nil, err
	}

	menu.UpdatedAt = time.Now()
	updated, err := s.menuRepo.Update(ctx, *menu)
	if err != nil {
		if err == apperrors.ErrConflict {
			return nil, mapMenuConflict(ctx, s.menuRepo, menu.ID, menu.Code, menu.Route)
		}
		return nil, err
	}
	return toMenuResponse(updated), nil
}

func (s *MenuService) Delete(ctx context.Context, id string) error {
	menuID := strings.TrimSpace(id)
	if menuID == "" {
		return apperrors.Validation("menu id is required")
	}

	if _, err := s.menuRepo.FindByID(ctx, menuID); err != nil {
		return err
	}

	hasChildren, err := s.menuRepo.HasChildren(ctx, menuID)
	if err != nil {
		return err
	}
	if hasChildren {
		return apperrors.ErrMenuHasChildren
	}
	return s.menuRepo.SoftDelete(ctx, menuID)
}

func (s *MenuService) Tree(ctx context.Context) ([]dto.MenuTreeResponse, error) {
	menus, err := s.menuRepo.ListActive(ctx)
	if err != nil {
		return nil, err
	}

	nodes := make(map[string]*menuTreeNode, len(menus))
	for _, menu := range menus {
		nodes[menu.ID] = &menuTreeNode{
			sortOrder: menu.SortOrder,
			response: dto.MenuTreeResponse{
				ID:        menu.ID,
				Code:      menu.Code,
				Name:      menu.Name,
				Route:     menu.Route,
				Icon:      menu.Icon,
				SortOrder: menu.SortOrder,
				Children:  []dto.MenuTreeResponse{},
			},
			children: []*menuTreeNode{},
		}
	}

	roots := make([]*menuTreeNode, 0)
	for _, menu := range menus {
		node := nodes[menu.ID]
		if menu.ParentID == nil {
			roots = append(roots, node)
			continue
		}

		parent, ok := nodes[*menu.ParentID]
		if !ok {
			continue
		}
		parent.children = append(parent.children, node)
	}

	sort.SliceStable(roots, func(i, j int) bool {
		return roots[i].sortOrder < roots[j].sortOrder
	})

	response := make([]dto.MenuTreeResponse, 0, len(roots))
	for _, root := range roots {
		response = append(response, buildTreeResponse(root))
	}
	return response, nil
}

func (s *MenuService) buildCreateMenu(ctx context.Context, req dto.CreateMenuRequest) (domain.Menu, error) {
	menuID, err := uuid.NewV7()
	if err != nil {
		return domain.Menu{}, fmt.Errorf("generate menu id: %w", err)
	}

	now := time.Now()
	menu := domain.Menu{
		ID:        menuID.String(),
		Code:      strings.TrimSpace(req.Code),
		Name:      strings.TrimSpace(req.Name),
		Route:     strings.TrimSpace(req.Route),
		SortOrder: req.SortOrder,
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if req.IsActive != nil {
		menu.IsActive = *req.IsActive
	}
	if parentID := strings.TrimSpace(req.ParentID); parentID != "" {
		menu.ParentID = &parentID
	}
	if icon := strings.TrimSpace(req.Icon); icon != "" {
		menu.Icon = &icon
	}

	if err := s.validateMenu(ctx, &menu, true); err != nil {
		return domain.Menu{}, err
	}
	return menu, nil
}

func (s *MenuService) validateMenu(ctx context.Context, menu *domain.Menu, creating bool) error {
	if menu == nil {
		return apperrors.Validation("menu is required")
	}
	if err := utils.ValidateMenuCode(menu.Code); err != nil {
		return apperrors.Validation(err.Error())
	}
	if strings.TrimSpace(menu.Name) == "" {
		return apperrors.Validation("name is required")
	}
	if menu.SortOrder < 0 {
		return apperrors.Validation("sort_order must be greater than or equal to 0")
	}

	menu.Route = strings.TrimSpace(menu.Route)
	if menu.ParentID != nil {
		parentID := strings.TrimSpace(*menu.ParentID)
		if parentID == "" {
			menu.ParentID = nil
		} else {
			if parentID == menu.ID {
				return apperrors.ErrMenuCycle
			}
			if err := s.validateParentChain(ctx, menu.ID, parentID); err != nil {
				return err
			}
			menu.ParentID = &parentID
		}
	}

	if menu.Route == "" {
		if !creating {
			hasChildren, err := s.menuRepo.HasChildren(ctx, menu.ID)
			if err != nil {
				return err
			}
			if !hasChildren {
				return apperrors.Validation("route is required for menus without children")
			}
		}
	} else if err := utils.ValidateMenuRoute(menu.Route); err != nil {
		return apperrors.Validation(err.Error())
	}

	if err := s.ensureUnique(ctx, menu.ID, menu.Code, menu.Route); err != nil {
		return err
	}
	return nil
}

func (s *MenuService) validateParentChain(ctx context.Context, currentID, parentID string) error {
	parent, err := s.menuRepo.FindByID(ctx, parentID)
	if err != nil {
		return apperrors.Validation("parent_id is invalid")
	}

	visited := map[string]struct{}{currentID: {}}
	for parent != nil {
		if _, exists := visited[parent.ID]; exists {
			return apperrors.ErrMenuCycle
		}
		visited[parent.ID] = struct{}{}
		if parent.ParentID == nil {
			return nil
		}
		parent, err = s.menuRepo.FindByID(ctx, *parent.ParentID)
		if err != nil {
			return apperrors.Validation("parent_id is invalid")
		}
	}
	return nil
}

func (s *MenuService) ensureUnique(ctx context.Context, currentID, code, route string) error {
	if code != "" {
		existing, err := s.menuRepo.FindByCode(ctx, code)
		if err != nil && err != apperrors.ErrMenuNotFound {
			return err
		}
		if err == nil && existing.ID != currentID {
			return apperrors.Validation("code already exists")
		}
	}
	if route != "" {
		existing, err := s.menuRepo.FindByRoute(ctx, route)
		if err != nil && err != apperrors.ErrMenuNotFound {
			return err
		}
		if err == nil && existing.ID != currentID {
			return apperrors.Validation("route already exists")
		}
	}
	return nil
}

func mapMenuConflict(ctx context.Context, repo repository.MenuRepository, currentID, code, route string) error {
	if code != "" {
		existing, err := repo.FindByCode(ctx, code)
		if err == nil && existing.ID != currentID {
			return apperrors.Validation("code already exists")
		}
	}
	if route != "" {
		existing, err := repo.FindByRoute(ctx, route)
		if err == nil && existing.ID != currentID {
			return apperrors.Validation("route already exists")
		}
	}
	return apperrors.ErrConflict
}

func toMenuResponse(menu *domain.Menu) *dto.MenuResponse {
	if menu == nil {
		return nil
	}
	return &dto.MenuResponse{
		ID:        menu.ID,
		Code:      menu.Code,
		Name:      menu.Name,
		Route:     menu.Route,
		Icon:      menu.Icon,
		ParentID:  menu.ParentID,
		SortOrder: menu.SortOrder,
		IsActive:  menu.IsActive,
	}
}

func buildTreeResponse(node *menuTreeNode) dto.MenuTreeResponse {
	if node == nil {
		return dto.MenuTreeResponse{}
	}

	response := node.response
	sort.SliceStable(node.children, func(i, j int) bool {
		return node.children[i].sortOrder < node.children[j].sortOrder
	})

	response.Children = make([]dto.MenuTreeResponse, 0, len(node.children))
	for _, child := range node.children {
		response.Children = append(response.Children, buildTreeResponse(child))
	}
	return response
}
