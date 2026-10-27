package application

import (
	"context"
	"strings"

	"task-api/internal/domain"
	"task-api/internal/domain/ports"
)

type TeamService struct {
	teams ports.TeamRepository
	users ports.UserRepository
}

func NewTeamService(teams ports.TeamRepository, users ports.UserRepository) *TeamService {
	return &TeamService{teams: teams, users: users}
}

func (s *TeamService) Create(ctx context.Context, userID int64, name string) (*domain.Team, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 150 {
		return nil, domain.ErrInvalid
	}
	team := &domain.Team{Name: name, CreatedBy: userID, Role: domain.RoleOwner}
	if err := s.teams.CreateAtomic(ctx, team); err != nil {
		return nil, err
	}
	return team, nil
}

func (s *TeamService) List(ctx context.Context, userID int64) ([]domain.Team, error) {
	return s.teams.ListForUser(ctx, userID)
}

func (s *TeamService) Invite(ctx context.Context, actorID, teamID, userID int64, role domain.Role) error {
	if role != domain.RoleMember && role != domain.RoleAdmin {
		return domain.ErrInvalid
	}
	actorRole, err := s.teams.RoleForUser(ctx, teamID, actorID)
	if err != nil {
		return err
	}
	if actorRole != domain.RoleOwner && actorRole != domain.RoleAdmin {
		return domain.ErrForbidden
	}
	if _, err := s.users.FindByID(ctx, userID); err != nil {
		return err
	}
	return s.teams.AddMemberAtomic(ctx, teamID, userID, role)
}

func (s *TeamService) SetMemberRole(ctx context.Context, actorID, teamID, userID int64, role domain.Role) error {
	if role != domain.RoleMember && role != domain.RoleAdmin {
		return domain.ErrInvalid
	}
	actorRole, err := s.teams.RoleForUser(ctx, teamID, actorID)
	if err != nil {
		return err
	}
	if actorRole != domain.RoleOwner {
		return domain.ErrForbidden
	}
	targetRole, err := s.teams.RoleForUser(ctx, teamID, userID)
	if err != nil {
		return err
	}
	if targetRole == domain.RoleOwner {
		return domain.ErrForbidden
	}
	if targetRole == role {
		return nil
	}
	return s.teams.UpdateMemberRole(ctx, teamID, userID, role)
}

func (s *TeamService) RemoveMember(ctx context.Context, actorID, teamID, userID int64) error {
	actorRole, err := s.teams.RoleForUser(ctx, teamID, actorID)
	if err != nil {
		return err
	}
	if actorRole != domain.RoleOwner && actorRole != domain.RoleAdmin {
		return domain.ErrForbidden
	}
	targetRole, err := s.teams.RoleForUser(ctx, teamID, userID)
	if err != nil {
		return err
	}
	if targetRole == domain.RoleOwner {
		return domain.ErrForbidden
	}
	return s.teams.RemoveMember(ctx, teamID, userID)
}
