package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

type fakeUserRepoForAuth struct {
	upsertResult *domain.User
	upsertErr    error
	upsertCalls  []struct{ supabaseUserID, email string }
}

func (f *fakeUserRepoForAuth) UpsertBySupabaseID(_ context.Context, supabaseUserID, email string) (*domain.User, error) {
	f.upsertCalls = append(f.upsertCalls, struct{ supabaseUserID, email string }{supabaseUserID, email})
	return f.upsertResult, f.upsertErr
}
func (f *fakeUserRepoForAuth) GetBySupabaseID(context.Context, string) (*domain.User, error) {
	return nil, nil
}
func (f *fakeUserRepoForAuth) GetByID(context.Context, string) (*domain.User, error)      { return nil, nil }
func (f *fakeUserRepoForAuth) SetStatus(context.Context, string, domain.UserStatus) error { return nil }
func (f *fakeUserRepoForAuth) SetPasswordChangeRequired(context.Context, string, bool) error {
	return nil
}
func (f *fakeUserRepoForAuth) IsActiveMemberOfTenant(context.Context, string, string) (bool, error) {
	return false, nil
}

func TestProvisionUser_Feliz(t *testing.T) {
	want := &domain.User{ID: "u1", Email: "x@example.com"}
	repo := &fakeUserRepoForAuth{upsertResult: want}
	uc := NewAuthUsecase(repo)

	got, err := uc.ProvisionUser(context.Background(), "supa-123", "x@example.com")

	assert.NoError(t, err)
	assert.Equal(t, want, got)
	assert.Equal(t, []struct{ supabaseUserID, email string }{{"supa-123", "x@example.com"}}, repo.upsertCalls)
}

func TestProvisionUser_ErrorDeRepoPropaga(t *testing.T) {
	repo := &fakeUserRepoForAuth{upsertErr: errors.New("db down")}
	uc := NewAuthUsecase(repo)

	_, err := uc.ProvisionUser(context.Background(), "supa-123", "x@example.com")

	assert.Error(t, err)
}
