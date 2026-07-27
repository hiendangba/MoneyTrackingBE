package httptransport

import (
	"net/http"

	authv1 "github.com/hiendangba/MoneyTrackingBE/Backend/auth-service/gen/auth/v1"
)

func (g *Gateway) ListMenus(w http.ResponseWriter, r *http.Request, claims AuthClaims) {
	ctx, cancel := requestContext(r, g.cfg.RequestTimeout)
	defer cancel()
	resp, err := g.authClient.ListMenus(g.upstreamContext(ctx, r, &claims), &authv1.ListMenusRequest{})
	if err != nil {
		writeError(w, err, g.logger)
		return
	}
	menus := make([]Menu, 0, len(resp.GetMenus()))
	for _, item := range resp.GetMenus() {
		menus = append(menus, toMenu(item))
	}
	writeJSON(w, http.StatusOK, menus)
}

func (g *Gateway) GetMenu(w http.ResponseWriter, r *http.Request, claims AuthClaims) {
	ctx, cancel := requestContext(r, g.cfg.RequestTimeout)
	defer cancel()
	resp, err := g.authClient.GetMenu(g.upstreamContext(ctx, r, &claims), &authv1.GetMenuRequest{
		Id: r.PathValue("id"),
	})
	if err != nil {
		writeError(w, err, g.logger)
		return
	}
	writeJSON(w, http.StatusOK, toMenu(resp))
}

func (g *Gateway) TreeMenus(w http.ResponseWriter, r *http.Request, claims AuthClaims) {
	ctx, cancel := requestContext(r, g.cfg.RequestTimeout)
	defer cancel()
	resp, err := g.authClient.TreeMenus(g.upstreamContext(ctx, r, &claims), &authv1.TreeMenusRequest{})
	if err != nil {
		writeError(w, err, g.logger)
		return
	}
	nodes := make([]MenuNode, 0, len(resp.GetNodes()))
	for _, item := range resp.GetNodes() {
		nodes = append(nodes, toMenuNode(item))
	}
	writeJSON(w, http.StatusOK, nodes)
}

func (g *Gateway) CreateMenu(w http.ResponseWriter, r *http.Request, claims AuthClaims) {
	var req CreateMenuRequest
	if err := g.decodeJSON(w, r, &req); err != nil {
		writeError(w, err, g.logger)
		return
	}
	ctx, cancel := requestContext(r, g.cfg.RequestTimeout)
	defer cancel()
	resp, err := g.authClient.CreateMenu(g.upstreamContext(ctx, r, &claims), &authv1.CreateMenuRequest{
		Code:      req.Code,
		Name:      req.Name,
		Route:     req.Route,
		Icon:      req.Icon,
		ParentId:  req.ParentID,
		SortOrder: int32(req.SortOrder),
		IsActive:  req.IsActive,
	})
	if err != nil {
		writeError(w, err, g.logger)
		return
	}
	writeJSON(w, http.StatusCreated, toMenu(resp))
}

func (g *Gateway) UpdateMenu(w http.ResponseWriter, r *http.Request, claims AuthClaims) {
	var req UpdateMenuRequest
	if err := g.decodeJSON(w, r, &req); err != nil {
		writeError(w, err, g.logger)
		return
	}
	ctx, cancel := requestContext(r, g.cfg.RequestTimeout)
	defer cancel()
	resp, err := g.authClient.UpdateMenu(g.upstreamContext(ctx, r, &claims), &authv1.UpdateMenuRequest{
		Id:        r.PathValue("id"),
		Code:      req.Code,
		Name:      req.Name,
		Route:     req.Route,
		Icon:      req.Icon,
		ParentId:  req.ParentID,
		SortOrder: int32Ptr(req.SortOrder),
		IsActive:  req.IsActive,
	})
	if err != nil {
		writeError(w, err, g.logger)
		return
	}
	writeJSON(w, http.StatusOK, toMenu(resp))
}

func (g *Gateway) DeleteMenu(w http.ResponseWriter, r *http.Request, claims AuthClaims) {
	ctx, cancel := requestContext(r, g.cfg.RequestTimeout)
	defer cancel()
	resp, err := g.authClient.DeleteMenu(g.upstreamContext(ctx, r, &claims), &authv1.DeleteMenuRequest{
		Id: r.PathValue("id"),
	})
	if err != nil {
		writeError(w, err, g.logger)
		return
	}
	writeJSON(w, http.StatusOK, MessageResponse{Message: resp.GetMessage()})
}

func toMenu(resp *authv1.Menu) Menu {
	if resp == nil {
		return Menu{}
	}
	var icon *string
	if resp.GetIcon() != "" {
		value := resp.GetIcon()
		icon = &value
	}
	var parentID *string
	if resp.GetParentId() != "" {
		value := resp.GetParentId()
		parentID = &value
	}
	return Menu{
		ID:        resp.GetId(),
		Code:      resp.GetCode(),
		Name:      resp.GetName(),
		Route:     resp.GetRoute(),
		Icon:      icon,
		ParentID:  parentID,
		SortOrder: int(resp.GetSortOrder()),
		IsActive:  resp.GetIsActive(),
	}
}

func toMenuNode(resp *authv1.MenuNode) MenuNode {
	if resp == nil {
		return MenuNode{}
	}
	children := make([]MenuNode, 0, len(resp.GetChildren()))
	for _, child := range resp.GetChildren() {
		children = append(children, toMenuNode(child))
	}
	var icon *string
	if resp.GetIcon() != "" {
		value := resp.GetIcon()
		icon = &value
	}
	return MenuNode{
		ID:        resp.GetId(),
		Code:      resp.GetCode(),
		Name:      resp.GetName(),
		Route:     resp.GetRoute(),
		Icon:      icon,
		SortOrder: int(resp.GetSortOrder()),
		Children:  children,
	}
}

func int32Ptr(value *int) *int32 {
	if value == nil {
		return nil
	}
	v := int32(*value)
	return &v
}
