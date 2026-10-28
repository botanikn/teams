package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"task-api/internal/application"
	"task-api/internal/domain"
	"task-api/internal/domain/ports"
)

type Handler struct {
	auth   *application.AuthService
	teams  *application.TeamService
	tasks  *application.TaskService
	tokens ports.TokenIssuer
}

func NewHandler(auth *application.AuthService, teams *application.TeamService, tasks *application.TaskService, tokens ports.TokenIssuer) *Handler {
	return &Handler{auth: auth, teams: teams, tasks: tasks, tokens: tokens}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /openapi.yaml", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		_, _ = w.Write(openAPISpec)
	})
	mux.HandleFunc("POST /api/v1/register", h.register)
	mux.HandleFunc("POST /api/v1/login", h.login)

	protected := func(pattern string, handler http.HandlerFunc) {
		mux.Handle(pattern, authenticate(h.tokens, handler))
	}
	protected("POST /api/v1/teams", h.createTeam)
	protected("GET /api/v1/teams", h.listTeams)
	protected("POST /api/v1/teams/{id}/invite", h.invite)
	protected("PUT /api/v1/teams/{id}/members/{user_id}", h.updateMemberRole)
	protected("DELETE /api/v1/teams/{id}/members/{user_id}", h.removeMember)
	protected("POST /api/v1/tasks", h.createTask)
	protected("GET /api/v1/tasks", h.listTasks)
	protected("PUT /api/v1/tasks/{id}", h.updateTask)
	protected("GET /api/v1/tasks/{id}/history", h.taskHistory)
	protected("POST /api/v1/tasks/{id}/comments", h.addComment)
	protected("GET /api/v1/tasks/{id}/comments", h.listComments)
	protected("GET /api/v1/teams/{team_id}/stats", h.teamStats)
	return mux
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Name     string `json:"name"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	user, err := h.auth.Register(r.Context(), request.Email, request.Password, request.Name)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toUserResponse(user))
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	token, err := h.auth.Login(r.Context(), request.Email, request.Password)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": token, "token_type": "Bearer"})
}

func (h *Handler) createTeam(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	team, err := h.teams.Create(r.Context(), currentUserID(r), request.Name)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toTeamResponse(*team))
}

func (h *Handler) listTeams(w http.ResponseWriter, r *http.Request) {
	teams, err := h.teams.List(r.Context(), currentUserID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toTeamResponses(teams))
}

func (h *Handler) invite(w http.ResponseWriter, r *http.Request) {
	teamID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var request struct {
		UserID int64       `json:"user_id"`
		Role   domain.Role `json:"role"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	if request.UserID <= 0 {
		writeError(w, domain.ErrInvalid)
		return
	}
	if err := h.teams.Invite(r.Context(), currentUserID(r), teamID, request.UserID, request.Role); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) updateMemberRole(w http.ResponseWriter, r *http.Request) {
	teamID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	userID, ok := pathID(w, r, "user_id")
	if !ok {
		return
	}
	var request struct {
		Role domain.Role `json:"role"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	if err := h.teams.SetMemberRole(r.Context(), currentUserID(r), teamID, userID, request.Role); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) removeMember(w http.ResponseWriter, r *http.Request) {
	teamID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	userID, ok := pathID(w, r, "user_id")
	if !ok {
		return
	}
	if err := h.teams.RemoveMember(r.Context(), currentUserID(r), teamID, userID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) createTask(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TeamID      int64  `json:"team_id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Status      string `json:"status"`
		AssigneeID  *int64 `json:"assignee_id"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	if request.TeamID <= 0 {
		writeError(w, domain.ErrInvalid)
		return
	}
	if request.Status == "" {
		request.Status = "todo"
	}
	task, err := h.tasks.Create(r.Context(), currentUserID(r), request.TeamID, request.Title, request.Description, request.Status, request.AssigneeID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toTaskResponse(task))
}

func (h *Handler) listTasks(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	teamID, err := queryInt64(query.Get("team_id"), 0)
	if err != nil || teamID <= 0 {
		writeError(w, domain.ErrInvalid)
		return
	}
	assigneeID, err := queryInt64(query.Get("assignee_id"), 0)
	if err != nil || assigneeID < 0 {
		writeError(w, domain.ErrInvalid)
		return
	}
	limit, err := queryInt(query.Get("limit"), 20)
	if err != nil {
		writeError(w, domain.ErrInvalid)
		return
	}
	offset, err := queryInt(query.Get("offset"), 0)
	if err != nil {
		writeError(w, domain.ErrInvalid)
		return
	}
	tasks, err := h.tasks.List(r.Context(), currentUserID(r), domain.TaskFilter{
		TeamID: teamID, Status: query.Get("status"), AssigneeID: assigneeID, Limit: limit, Offset: offset,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toTaskResponses(tasks))
}

func (h *Handler) updateTask(w http.ResponseWriter, r *http.Request) {
	taskID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var request struct {
		Version     int64   `json:"version"`
		Title       *string `json:"title"`
		Description *string `json:"description"`
		Status      *string `json:"status"`
		AssigneeID  *int64  `json:"assignee_id"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	task, err := h.tasks.Update(r.Context(), currentUserID(r), taskID, request.Version, domain.TaskChange{
		Title: request.Title, Description: request.Description, Status: request.Status, AssigneeID: request.AssigneeID,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toTaskResponse(task))
}

func (h *Handler) taskHistory(w http.ResponseWriter, r *http.Request) {
	taskID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	items, err := h.tasks.History(r.Context(), currentUserID(r), taskID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toTaskHistoryResponses(items))
}

func (h *Handler) addComment(w http.ResponseWriter, r *http.Request) {
	taskID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var request struct {
		Content string `json:"content"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	comment, err := h.tasks.AddComment(r.Context(), currentUserID(r), taskID, request.Content)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toTaskCommentResponse(comment))
}

func (h *Handler) listComments(w http.ResponseWriter, r *http.Request) {
	taskID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	items, err := h.tasks.Comments(r.Context(), currentUserID(r), taskID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toTaskCommentResponses(items))
}

func (h *Handler) teamStats(w http.ResponseWriter, r *http.Request) {
	teamID, ok := pathID(w, r, "team_id")
	if !ok {
		return
	}
	stats, err := h.tasks.Stats(r.Context(), currentUserID(r), teamID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toTeamStatsResponse(stats))
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "request body must contain one JSON object"})
		return false
	}
	return true
}

func pathID(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, domain.ErrInvalid)
		return 0, false
	}
	return id, true
}

func queryInt64(value string, fallback int64) (int64, error) {
	if value == "" {
		return fallback, nil
	}
	return strconv.ParseInt(value, 10, 64)
}

func queryInt(value string, fallback int) (int, error) {
	if value == "" {
		return fallback, nil
	}
	return strconv.Atoi(value)
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	message := "internal server error"
	switch {
	case errors.Is(err, domain.ErrInvalid):
		status, message = http.StatusBadRequest, "invalid input"
	case errors.Is(err, domain.ErrUnauthenticated):
		status, message = http.StatusUnauthorized, "authentication required"
	case errors.Is(err, domain.ErrForbidden):
		status, message = http.StatusForbidden, "access denied"
	case errors.Is(err, domain.ErrNotFound):
		status, message = http.StatusNotFound, "resource not found"
	case errors.Is(err, domain.ErrConflict):
		status, message = http.StatusConflict, "resource conflict or stale version"
	default:
		slog.Error("request failed", "error", err)
	}
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Error("encode HTTP response", "error", err)
	}
}
