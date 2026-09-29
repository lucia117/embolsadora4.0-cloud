package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

type fakeUserRepoForClearPwd struct {
	setPasswordChangeRequiredErr   error
	setPasswordChangeRequiredCalls []struct {
		userID string
		value  bool
	}
}

func (f *fakeUserRepoForClearPwd) UpsertBySupabaseID(context.Context, string, string) (*domain.User, error) {
	return nil, nil
}
func (f *fakeUserRepoForClearPwd) GetBySupabaseID(context.Context, string) (*domain.User, error) {
	return nil, nil
}
func (f *fakeUserRepoForClearPwd) GetByID(context.Context, string) (*domain.User, error) {
	return nil, nil
}
func (f *fakeUserRepoForClearPwd) SetStatus(context.Context, string, domain.UserStatus) error {
	return nil
}
func (f *fakeUserRepoForClearPwd) SetPasswordChangeRequired(_ context.Context, userID string, value bool) error {
	f.setPasswordChangeRequiredCalls = append(f.setPasswordChangeRequiredCalls, struct {
		userID string
		value  bool
	}{userID, value})
	return f.setPasswordChangeRequiredErr
}
func (f *fakeUserRepoForClearPwd) IsActiveMemberOfTenant(context.Context, string, string) (bool, error) {
	return false, nil
}

func TestClearPasswordChangeRequired_SinDomainUserEnContextoEsForbidden(t *testing.T) {
	uc := NewPasswordUsecase(&fakeUserRepoForClearPwd{}, nil, nil, "", nil)

	err := uc.ClearPasswordChangeRequired(context.Background())

	assert.ErrorIs(t, err, domain.ErrForbidden)
}

func TestClearPasswordChangeRequired_Feliz(t *testing.T) {
	repo := &fakeUserRepoForClearPwd{}
	uc := NewPasswordUsecase(repo, nil, nil, "", nil)
	ctx := platform.WithDomainUser(context.Background(), &domain.User{ID: "u1"})

	err := uc.ClearPasswordChangeRequired(ctx)

	assert.NoError(t, err)
	assert.Equal(t, []struct {
		userID string
		value  bool
	}{{"u1", false}}, repo.setPasswordChangeRequiredCalls)
}

func TestClearPasswordChangeRequired_ErrorDeRepoPropaga(t *testing.T) {
	repo := &fakeUserRepoForClearPwd{setPasswordChangeRequiredErr: errors.New("db down")}
	uc := NewPasswordUsecase(repo, nil, nil, "", nil)
	ctx := platform.WithDomainUser(context.Background(), &domain.User{ID: "u1"})

	err := uc.ClearPasswordChangeRequired(ctx)

	assert.Error(t, err)
}
