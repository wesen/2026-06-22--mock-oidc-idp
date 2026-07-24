package idpadminapp

import (
	"context"
	"crypto/sha256"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-go-golems/tiny-idp/pkg/idpadmin"
	"github.com/go-go-golems/tiny-idp/pkg/idpadminstore"
)

const diagnosticsDownloadLifetime = 10 * time.Minute

type DownloadGrant struct {
	Handle       string    `json:"download_handle"`
	DownloadName string    `json:"download_name"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type ConsumedDownload struct {
	Path         string
	ContentType  string
	DownloadName string
}

type DownloadService struct {
	store      idpadminstore.Store
	authorizer *idpadmin.Authorizer
	root       string
	now        func() time.Time
	id         func() (string, error)
}

func NewDownloadService(
	store idpadminstore.Store,
	authorizer *idpadmin.Authorizer,
	root string,
	now func() time.Time,
) (*DownloadService, error) {
	if store == nil || authorizer == nil {
		return nil, errors.New("download store and authorizer are required")
	}
	canonical, err := prepareManagedRoot(root)
	if err != nil {
		return nil, err
	}
	if now == nil {
		now = time.Now
	}
	return &DownloadService{
		store: store, authorizer: authorizer, root: canonical, now: now, id: randomID,
	}, nil
}

func (s *DownloadService) Issue(
	ctx context.Context,
	principal idpadmin.AdminPrincipal,
	operationID string,
) (DownloadGrant, error) {
	if _, err := s.authorizer.Authorize(
		ctx, principal, principal.GrantID, principal.GrantVersion,
		idpadmin.SystemScope(), idpadmin.CapabilityDiagnosticsRead, false,
	); err != nil {
		return DownloadGrant{}, err
	}
	operation, err := s.store.GetAdminOperation(ctx, strings.TrimSpace(operationID))
	if err != nil {
		return DownloadGrant{}, err
	}
	if operation.Command != CommandDiagnosticsCreate || operation.Status != "completed" ||
		operation.RelativeResultPath == "" {
		return DownloadGrant{}, idpadminstore.ErrNotFound
	}
	if _, err := s.resolveExisting(operation.RelativeResultPath); err != nil {
		return DownloadGrant{}, err
	}
	first, err := s.id()
	if err != nil {
		return DownloadGrant{}, err
	}
	second, err := s.id()
	if err != nil {
		return DownloadGrant{}, err
	}
	handle := first + second
	hash := sha256.Sum256([]byte(handle))
	now := s.now().UTC()
	expires := now.Add(diagnosticsDownloadLifetime)
	name := "tinyidp-diagnostics-" + operation.ID + ".json"
	if err := s.store.CreateAdminDownload(ctx, idpadminstore.DownloadRecord{
		HandleHash: hash[:], OperationID: operation.ID,
		RelativePath: operation.RelativeResultPath, ContentType: "application/json",
		DownloadName: name, CreatedAt: now, ExpiresAt: expires,
	}); err != nil {
		return DownloadGrant{}, err
	}
	return DownloadGrant{Handle: handle, DownloadName: name, ExpiresAt: expires}, nil
}

func (s *DownloadService) Consume(
	ctx context.Context,
	principal idpadmin.AdminPrincipal,
	handle string,
) (ConsumedDownload, error) {
	if _, err := s.authorizer.Authorize(
		ctx, principal, principal.GrantID, principal.GrantVersion,
		idpadmin.SystemScope(), idpadmin.CapabilityDiagnosticsRead, false,
	); err != nil {
		return ConsumedDownload{}, err
	}
	hash := sha256.Sum256([]byte(strings.TrimSpace(handle)))
	record, err := s.store.ConsumeAdminDownload(ctx, hash[:], s.now().UTC())
	if err != nil {
		return ConsumedDownload{}, err
	}
	path, err := s.resolveExisting(record.RelativePath)
	if err != nil {
		return ConsumedDownload{}, err
	}
	return ConsumedDownload{
		Path: path, ContentType: record.ContentType, DownloadName: record.DownloadName,
	}, nil
}

func (s *DownloadService) resolveExisting(relative string) (string, error) {
	runner := ManagedOperationRunner{root: s.root}
	return runner.resolveExisting(filepath.ToSlash(relative))
}
