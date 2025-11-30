package websocket

import (
	"context"

	"github.com/google/uuid"
	"github.com/rx3lixir/laba_zis/internal/user"
)

// UserStoreAdapter adapts user.Store to websocket.UserStore
type UserStoreAdapter struct {
	store user.Store
}

// NewUserStoreAdapter creates a new adapter
func NewUserStoreAdapter(store user.Store) *UserStoreAdapter {
	return &UserStoreAdapter{store: store}
}

// GetUserByID implements websocket.UserStore interface
func (a *UserStoreAdapter) GetUserByID(ctx context.Context, id uuid.UUID) (*UserInfo, error) {
	u, err := a.store.GetUserByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return &UserInfo{
		ID:       u.ID,
		Username: u.Username,
	}, nil
}
