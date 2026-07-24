package idpadminapp

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-go-golems/tiny-idp/pkg/idpadmin"
	"github.com/go-go-golems/tiny-idp/pkg/idpadminstore"
	"github.com/go-go-golems/tiny-idp/pkg/idpstore"
)

var ErrUnknownAdminPage = errors.New("unknown administration page")

type PageStore interface {
	idpadminstore.Store
	idpstore.Store
}

type PageDataService struct {
	store      PageStore
	authorizer *idpadmin.Authorizer
	now        func() time.Time
}

func NewPageDataService(store PageStore, authorizer *idpadmin.Authorizer, now func() time.Time) (*PageDataService, error) {
	if store == nil || authorizer == nil {
		return nil, errors.New("page data store and authorizer are required")
	}
	if now == nil {
		now = time.Now
	}
	return &PageDataService{store: store, authorizer: authorizer, now: now}, nil
}

func (s *PageDataService) PageData(
	ctx context.Context,
	principal idpadmin.AdminPrincipal,
	pageID string,
	query url.Values,
) (map[string]any, error) {
	switch pageID {
	case "", "overview":
		if err := s.authorize(ctx, principal, idpadmin.CapabilityOverviewRead); err != nil {
			return nil, err
		}
		return s.overview(ctx)
	case "users":
		if err := s.authorize(ctx, principal, idpadmin.CapabilityUsersRead); err != nil {
			return nil, err
		}
		return s.users(ctx, query)
	case "invitations":
		if err := s.authorize(ctx, principal, idpadmin.CapabilityInvitationsRead); err != nil {
			return nil, err
		}
		return s.invitations(ctx)
	case "clients":
		if err := s.authorize(ctx, principal, idpadmin.CapabilityClientsRead); err != nil {
			return nil, err
		}
		return s.clients(ctx)
	case "keys":
		if err := s.authorize(ctx, principal, idpadmin.CapabilityKeysRead); err != nil {
			return nil, err
		}
		return s.keys(ctx)
	case "activity":
		if err := s.authorize(ctx, principal, idpadmin.CapabilityActivityRead); err != nil {
			return nil, err
		}
		return s.activity(ctx)
	case "operations":
		if err := s.authorize(ctx, principal, idpadmin.CapabilityOperationsRead); err != nil {
			return nil, err
		}
		return s.operations(ctx)
	default:
		return nil, ErrUnknownAdminPage
	}
}

func (s *PageDataService) authorize(ctx context.Context, principal idpadmin.AdminPrincipal, capability idpadmin.Capability) error {
	if principal.GrantID == "" || principal.GrantVersion < 1 {
		return idpadmin.ErrInvalidPrincipal
	}
	_, err := s.authorizer.Authorize(
		ctx, principal, principal.GrantID, principal.GrantVersion,
		idpadmin.SystemScope(), capability, false,
	)
	return err
}

func (s *PageDataService) overview(ctx context.Context) (map[string]any, error) {
	overview, err := s.store.GetAdminOverview(ctx, s.now().UTC())
	if err != nil {
		return nil, err
	}
	return pageData("overview", "Overview", []map[string]string{
		{"label": "Users", "value": strconv.FormatInt(overview.UserCount, 10)},
		{"label": "Disabled", "value": strconv.FormatInt(overview.DisabledUserCount, 10)},
		{"label": "Clients", "value": strconv.FormatInt(overview.ActiveClientCount, 10)},
		{"label": "Invitations", "value": strconv.FormatInt(overview.PendingInvitationCount, 10)},
	}, nil, "Control plane", fmt.Sprintf("Schema %d · %d pending operations", overview.SchemaVersion, overview.PendingOperationCount)), nil
}

func (s *PageDataService) users(ctx context.Context, query url.Values) (map[string]any, error) {
	status := idpadmin.UserStatus(query.Get("status"))
	rows, err := s.store.ListAdminUsers(ctx, idpadmin.UserFilter{Query: query.Get("q"), Status: status}, pageLimit(query))
	if err != nil {
		return nil, err
	}
	items := make([]map[string]string, 0, len(rows))
	for _, row := range rows {
		state := "active"
		if row.Disabled {
			state = "disabled"
		} else if row.LockedUntil != nil {
			state = "locked"
		}
		items = append(items, map[string]string{
			"id": row.ID, "primary": firstNonempty(row.DisplayName, row.Login),
			"secondary": firstNonempty(row.Email, row.Login), "status": state,
		})
	}
	return pageData("users", "Users", []map[string]string{
		{"label": "Visible", "value": strconv.Itoa(len(items))},
	}, items, "Read-only users", "Search and status filters are enforced by typed server queries."), nil
}

func (s *PageDataService) invitations(ctx context.Context) (map[string]any, error) {
	rows, err := s.store.ListAdminInvitations(ctx, s.now().UTC(), idpadmin.DefaultPageSize)
	if err != nil {
		return nil, err
	}
	items := make([]map[string]string, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]string{
			"id": row.ID, "primary": firstNonempty(row.Label, row.ID),
			"secondary": row.Audience, "status": row.Status,
		})
	}
	return pageData("invitations", "Invitations", nil, items, "Invitation lifecycle", "One-time invitation codes are never returned by this read model."), nil
}

func (s *PageDataService) clients(ctx context.Context) (map[string]any, error) {
	clients, err := s.store.ListClients(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]map[string]string, 0, min(len(clients), idpadmin.MaxPageSize))
	for _, client := range clients {
		if len(items) == idpadmin.MaxPageSize {
			break
		}
		status := "active"
		if client.Disabled {
			status = "disabled"
		}
		kind := "confidential"
		if client.Public {
			kind = "public"
		}
		items = append(items, map[string]string{
			"id": client.ID, "primary": client.ID, "secondary": kind, "status": status,
		})
	}
	return pageData("clients", "Applications", nil, items, "OIDC clients", "Client secrets and hashes are excluded."), nil
}

func (s *PageDataService) keys(ctx context.Context) (map[string]any, error) {
	keys, err := s.store.VerificationKeys(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]map[string]string, 0, len(keys))
	for _, key := range keys {
		status := "verification"
		if key.Active {
			status = "active"
		}
		items = append(items, map[string]string{
			"id": key.ID, "primary": key.ID, "secondary": key.Algorithm, "status": status,
		})
	}
	return pageData("keys", "Signing keys", nil, items, "Public lifecycle metadata", "Private key material is never projected."), nil
}

func (s *PageDataService) activity(ctx context.Context) (map[string]any, error) {
	rows, err := s.store.ListAdminActivity(ctx, idpadmin.DefaultPageSize)
	if err != nil {
		return nil, err
	}
	items := make([]map[string]string, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]string{
			"id": row.ID, "primary": row.Command,
			"secondary": strings.TrimSpace(row.TargetType + " " + row.TargetID), "status": row.Result,
		})
	}
	return pageData("activity", "Activity", nil, items, "Administrative evidence", "Newest successful and failed control-plane actions."), nil
}

func (s *PageDataService) operations(ctx context.Context) (map[string]any, error) {
	view, err := s.store.ListAdminOperations(ctx, idpadmin.DefaultPageSize)
	if err != nil {
		return nil, err
	}
	items := make([]map[string]string, 0, len(view.Operations))
	for _, row := range view.Operations {
		items = append(items, map[string]string{
			"id": row.ID, "primary": row.Kind, "secondary": row.ErrorCode, "status": row.Status,
		})
	}
	return pageData("operations", "Operations", []map[string]string{
		{"label": "Pending audit", "value": strconv.FormatInt(view.PendingOutbox, 10)},
	}, items, "Background operations", "Long-running work and audit delivery state."), nil
}

func pageData(
	id, title string,
	metrics []map[string]string,
	rows []map[string]string,
	messageTitle, message string,
) map[string]any {
	return map[string]any{
		"id": id, "title": title, "metrics": metrics, "rows": rows,
		"messageTitle": messageTitle, "message": message, "tone": "info",
		"tableTitle": title, "primaryLabel": "Name", "secondaryLabel": "Details",
	}
}

func pageLimit(query url.Values) int {
	value, err := strconv.Atoi(query.Get("limit"))
	if err != nil {
		return idpadmin.DefaultPageSize
	}
	return (idpadmin.CursorPageRequest{Limit: value}).Normalized().Limit
}

func firstNonempty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
