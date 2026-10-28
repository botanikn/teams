package application

import (
	"context"
	"strings"

	"task-api/internal/domain/entities"
	"task-api/internal/domain/errrors"
	"task-api/internal/domain/ports"
)

type TeamService struct {
	teams ports.TeamRepository
	users ports.UserRepository
}

func NewTeamService(teams ports.TeamRepository, users ports.UserRepository) *TeamService {
	return &TeamService{teams: teams, users: users}
}

func (s *TeamService) Create(ctx context.Context, userID int64, name string) (*entities.Team, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 150 {
		return nil, errrors.ErrInvalid
	}
	team := &entities.Team{Name: name, CreatedBy: userID}
	if err := s.teams.CreateAtomic(ctx, team); err != nil {
		return nil, err
	}
	return team, nil
}

func (s *TeamService) List(ctx context.Context, userID int64) ([]entities.Team, error) {
	return s.teams.ListForUser(ctx, userID)
}

func (s *TeamService) Invite(ctx context.Context, actorID, teamID, userID int64, role entities.Role) error {
	if role != entities.RoleMember && role != entities.RoleAdmin {
		return errrors.ErrInvalid
	}
	actorRole, err := s.teams.RoleForUser(ctx, teamID, actorID)
	if err != nil {
		return err
	}
	if actorRole != entities.RoleOwner && actorRole != entities.RoleAdmin {
		return errrors.ErrForbidden
	}
	if _, err := s.users.FindByID(ctx, userID); err != nil {
		return err
	}
	return s.teams.AddMemberAtomic(ctx, teamID, userID, role)
}

func (s *TeamService) SetMemberRole(ctx context.Context, actorID, teamID, userID int64, role entities.Role) error {
	if role != entities.RoleMember && role != entities.RoleAdmin {
		return errrors.ErrInvalid
	}
	actorRole, err := s.teams.RoleForUser(ctx, teamID, actorID)
	if err != nil {
		return err
	}
	if actorRole != entities.RoleOwner {
		return errrors.ErrForbidden
	}
	targetRole, err := s.teams.RoleForUser(ctx, teamID, userID)
	if err != nil {
		return err
	}
	if targetRole == entities.RoleOwner {
		return errrors.ErrForbidden
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
	if actorRole != entities.RoleOwner && actorRole != entities.RoleAdmin {
		return errrors.ErrForbidden
	}
	targetRole, err := s.teams.RoleForUser(ctx, teamID, userID)
	if err != nil {
		return err
	}
	if targetRole == entities.RoleOwner {
		return errrors.ErrForbidden
	}
	return s.teams.RemoveMember(ctx, teamID, userID)
}
