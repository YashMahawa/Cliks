package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

const (
	codeAlphabet            = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	dummyDeletePasswordHash = "$2b$12$mMCOaGsqrw5HVe4PboZEdeKqkZZSrer3p4/KmwcbXB0YraVftIwf."
)

type Team struct {
	ID        string `json:"id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
}

type CreateTeamInput struct {
	Name           string
	DeletePassword string
}

type DeleteTeamInput struct {
	Code           string
	DeletePassword string
}

type TeamStore interface {
	CreateTeam(context.Context, CreateTeamInput) (Team, error)
	GetTeamByCode(context.Context, string) (*Team, error)
	DeleteTeam(context.Context, DeleteTeamInput) (bool, error)
}

func createTeamStoreFromEnv() (TeamStore, error) {
	if supabaseURL := os.Getenv("SUPABASE_URL"); supabaseURL != "" && os.Getenv("SUPABASE_SERVICE_ROLE_KEY") != "" {
		return NewSupabaseTeamStore(supabaseURL, os.Getenv("SUPABASE_SERVICE_ROLE_KEY")), nil
	}
	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		return NewPostgresTeamStore(databaseURL)
	}
	if os.Getenv("CLIKS_LOCAL_POSTGRES") == "true" {
		return NewPostgresTeamStore("user=cliks dbname=cliks host=/var/run/postgresql sslmode=disable")
	}
	return NewMemoryTeamStore(), nil
}

type PostgresTeamStore struct {
	db *sql.DB
}

func NewPostgresTeamStore(dsn string) (*PostgresTeamStore, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	store := &PostgresTeamStore{db: db}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := store.init(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *PostgresTeamStore) init(ctx context.Context) error {
	queries := []string{
		`create table if not exists cliks_teams (
			id uuid primary key,
			code text not null,
			name text not null,
			delete_password_hash text not null,
			created_at timestamptz not null default now(),
			deleted_at timestamptz
		)`,
		`alter table cliks_teams drop constraint if exists cliks_teams_code_key`,
		`create unique index if not exists cliks_teams_code_active_idx
			on cliks_teams (code)
			where deleted_at is null`,
	}
	for _, query := range queries {
		if _, err := s.db.ExecContext(ctx, query); err != nil {
			return err
		}
	}
	return nil
}

func (s *PostgresTeamStore) CreateTeam(ctx context.Context, input CreateTeamInput) (Team, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(input.DeletePassword), 12)
	if err != nil {
		return Team{}, err
	}
	for attempt := 0; attempt < 8; attempt++ {
		id := newUUID()
		code := makeCode()
		var row postgresTeamRow
		err := s.db.QueryRowContext(ctx,
			`insert into cliks_teams (id, code, name, delete_password_hash)
			 values ($1, $2, $3, $4)
			 returning id, code, name, created_at`,
			id, code, input.Name, string(hash),
		).Scan(&row.ID, &row.Code, &row.Name, &row.CreatedAt)
		if err == nil {
			return row.toTeam(), nil
		}
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			continue
		}
		return Team{}, err
	}
	return Team{}, errors.New("could not generate a unique team code")
}

func (s *PostgresTeamStore) GetTeamByCode(ctx context.Context, code string) (*Team, error) {
	var row postgresTeamRow
	err := s.db.QueryRowContext(ctx,
		`select id, code, name, created_at
		 from cliks_teams
		 where code = $1 and deleted_at is null
		 limit 1`,
		normalizeTeamCode(code),
	).Scan(&row.ID, &row.Code, &row.Name, &row.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	team := row.toTeam()
	return &team, nil
}

func (s *PostgresTeamStore) DeleteTeam(ctx context.Context, input DeleteTeamInput) (bool, error) {
	var id string
	var hash string
	err := s.db.QueryRowContext(ctx,
		`select id, delete_password_hash
		 from cliks_teams
		 where code = $1 and deleted_at is null
		 limit 1`,
		normalizeTeamCode(input.Code),
	).Scan(&id, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		_ = bcrypt.CompareHashAndPassword([]byte(dummyDeletePasswordHash), []byte(input.DeletePassword))
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(input.DeletePassword)) != nil {
		return false, nil
	}
	_, err = s.db.ExecContext(ctx, "update cliks_teams set deleted_at = now() where id = $1", id)
	return err == nil, err
}

type postgresTeamRow struct {
	ID        string
	Code      string
	Name      string
	CreatedAt time.Time
}

func (r postgresTeamRow) toTeam() Team {
	return Team{
		ID:        r.ID,
		Code:      r.Code,
		Name:      r.Name,
		CreatedAt: r.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
}

type MemoryTeamStore struct {
	mu    sync.Mutex
	teams map[string]memoryTeam
}

type memoryTeam struct {
	Team
	DeletePasswordHash string
	DeletedAt          string
}

func NewMemoryTeamStore() *MemoryTeamStore {
	hash, _ := bcrypt.GenerateFromPassword([]byte("delete-me"), 12)
	store := &MemoryTeamStore{teams: map[string]memoryTeam{}}
	store.teams["CLIK-LOCAL"] = memoryTeam{
		Team: Team{
			ID:        newUUID(),
			Code:      "CLIK-LOCAL",
			Name:      "Local Test Room",
			CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
		},
		DeletePasswordHash: string(hash),
	}
	return store
}

func (s *MemoryTeamStore) CreateTeam(ctx context.Context, input CreateTeamInput) (Team, error) {
	_ = ctx
	hash, err := bcrypt.GenerateFromPassword([]byte(input.DeletePassword), 12)
	if err != nil {
		return Team{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	code := makeCode()
	for s.teams[code].DeletedAt == "" && s.teams[code].Code != "" {
		code = makeCode()
	}
	team := Team{ID: newUUID(), Code: code, Name: input.Name, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	s.teams[code] = memoryTeam{Team: team, DeletePasswordHash: string(hash)}
	return team, nil
}

func (s *MemoryTeamStore) GetTeamByCode(ctx context.Context, code string) (*Team, error) {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	team := s.teams[normalizeTeamCode(code)]
	if team.Code == "" || team.DeletedAt != "" {
		return nil, nil
	}
	out := team.Team
	return &out, nil
}

func (s *MemoryTeamStore) DeleteTeam(ctx context.Context, input DeleteTeamInput) (bool, error) {
	_ = ctx
	s.mu.Lock()
	team := s.teams[normalizeTeamCode(input.Code)]
	s.mu.Unlock()
	hash := dummyDeletePasswordHash
	if team.Code != "" && team.DeletedAt == "" {
		hash = team.DeletePasswordHash
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(input.DeletePassword)) != nil {
		return false, nil
	}
	if team.Code == "" || team.DeletedAt != "" {
		return false, nil
	}
	s.mu.Lock()
	team.DeletedAt = time.Now().UTC().Format(time.RFC3339Nano)
	s.teams[normalizeTeamCode(input.Code)] = team
	s.mu.Unlock()
	return true, nil
}

type SupabaseTeamStore struct {
	baseURL string
	key     string
	client  *http.Client
}

func NewSupabaseTeamStore(baseURL string, key string) *SupabaseTeamStore {
	return &SupabaseTeamStore{
		baseURL: strings.TrimRight(baseURL, "/"),
		key:     key,
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (s *SupabaseTeamStore) CreateTeam(ctx context.Context, input CreateTeamInput) (Team, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(input.DeletePassword), 12)
	if err != nil {
		return Team{}, err
	}
	for attempt := 0; attempt < 8; attempt++ {
		code := makeCode()
		var rows []supabaseTeamRow
		err := s.rest(ctx, http.MethodPost, "/rest/v1/cliks_teams?select=id,code,name,created_at", map[string]string{
			"code":                 code,
			"name":                 input.Name,
			"delete_password_hash": string(hash),
		}, &rows, "return=representation")
		if err == nil && len(rows) > 0 {
			team := rows[0].toTeam()
			return team, nil
		}
		if err != nil && strings.Contains(err.Error(), "23505") {
			continue
		}
		if err != nil {
			return Team{}, err
		}
	}
	return Team{}, errors.New("could not generate a unique team code")
}

func (s *SupabaseTeamStore) GetTeamByCode(ctx context.Context, code string) (*Team, error) {
	query := fmt.Sprintf("/rest/v1/cliks_teams?select=id,code,name,created_at&code=eq.%s&deleted_at=is.null&limit=1", url.QueryEscape(normalizeTeamCode(code)))
	var rows []supabaseTeamRow
	if err := s.rest(ctx, http.MethodGet, query, nil, &rows, ""); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	team := rows[0].toTeam()
	return &team, nil
}

func (s *SupabaseTeamStore) DeleteTeam(ctx context.Context, input DeleteTeamInput) (bool, error) {
	query := fmt.Sprintf("/rest/v1/cliks_teams?select=id,delete_password_hash&code=eq.%s&deleted_at=is.null&limit=1", url.QueryEscape(normalizeTeamCode(input.Code)))
	var rows []struct {
		ID                 string `json:"id"`
		DeletePasswordHash string `json:"delete_password_hash"`
	}
	if err := s.rest(ctx, http.MethodGet, query, nil, &rows, ""); err != nil {
		return false, err
	}
	hash := dummyDeletePasswordHash
	if len(rows) > 0 {
		hash = rows[0].DeletePasswordHash
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(input.DeletePassword)) != nil {
		return false, nil
	}
	if len(rows) == 0 {
		return false, nil
	}
	patch := fmt.Sprintf("/rest/v1/cliks_teams?id=eq.%s", url.QueryEscape(rows[0].ID))
	return true, s.rest(ctx, http.MethodPatch, patch, map[string]string{"deleted_at": time.Now().UTC().Format(time.RFC3339Nano)}, nil, "")
}

func (s *SupabaseTeamStore) rest(ctx context.Context, method string, path string, input any, output any, prefer string) error {
	var body io.Reader
	if input != nil {
		data, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, s.baseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("apikey", s.key)
	req.Header.Set("Authorization", "Bearer "+s.key)
	req.Header.Set("Content-Type", "application/json")
	if prefer != "" {
		req.Header.Set("Prefer", prefer)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxRequestBodySize))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return formatHTTPError(resp.StatusCode, data)
	}
	if output != nil && len(data) > 0 {
		return json.Unmarshal(data, output)
	}
	return nil
}

type supabaseTeamRow struct {
	ID        string `json:"id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

func (r supabaseTeamRow) toTeam() Team {
	created := r.CreatedAt
	if parsed, err := time.Parse(time.RFC3339Nano, created); err == nil {
		created = parsed.UTC().Format(time.RFC3339Nano)
	}
	return Team{ID: r.ID, Code: r.Code, Name: r.Name, CreatedAt: created}
}

func makeCode() string {
	var bytes [6]byte
	_, _ = rand.Read(bytes[:])
	var builder strings.Builder
	builder.WriteString("CLIK-")
	for _, b := range bytes {
		builder.WriteByte(codeAlphabet[int(b)%len(codeAlphabet)])
	}
	return builder.String()
}

func newUUID() string {
	var bytes [16]byte
	_, _ = rand.Read(bytes[:])
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(bytes[0:4]),
		hex.EncodeToString(bytes[4:6]),
		hex.EncodeToString(bytes[6:8]),
		hex.EncodeToString(bytes[8:10]),
		hex.EncodeToString(bytes[10:16]),
	)
}
