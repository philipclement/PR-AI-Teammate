package storage

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/example/pr-ai-teammate/internal/analysis"
	_ "github.com/lib/pq"
)

type Store interface {
	UpsertPullRequest(ctx context.Context, repo string, number int, sha string, title string, status string) (int64, error)
	UpdatePullRequestStatus(ctx context.Context, id int64, status string) error
	SaveAnalysisResults(ctx context.Context, prID int64, issues []analysis.Issue) error
}

func NewStore(ctx context.Context, dsn string) (Store, error) {
	if dsn == "" {
		return NewMemoryStore(), nil
	}
	return NewPostgresStore(ctx, dsn)
}

type MemoryStore struct {
	mu        sync.Mutex
	nextID    int64
	pulls     map[string]*pullRequestRecord
	analyses  map[int64][]analysis.Issue
	updatedAt time.Time
}

type pullRequestRecord struct {
	ID        int64
	Repo      string
	Number    int
	SHA       string
	Title     string
	Status    string
	CreatedAt time.Time
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		nextID:   1,
		pulls:    make(map[string]*pullRequestRecord),
		analyses: make(map[int64][]analysis.Issue),
	}
}

func (m *MemoryStore) UpsertPullRequest(ctx context.Context, repo string, number int, sha string, title string, status string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%s#%d", repo, number)
	if existing, ok := m.pulls[key]; ok {
		existing.SHA = sha
		existing.Title = title
		existing.Status = status
		return existing.ID, nil
	}
	id := m.nextID
	m.nextID++
	m.pulls[key] = &pullRequestRecord{
		ID:        id,
		Repo:      repo,
		Number:    number,
		SHA:       sha,
		Title:     title,
		Status:    status,
		CreatedAt: time.Now().UTC(),
	}
	return id, nil
}

func (m *MemoryStore) UpdatePullRequestStatus(ctx context.Context, id int64, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, pr := range m.pulls {
		if pr.ID == id {
			pr.Status = status
			return nil
		}
	}
	return fmt.Errorf("pull request not found")
}

func (m *MemoryStore) SaveAnalysisResults(ctx context.Context, prID int64, issues []analysis.Issue) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.analyses[prID] = append([]analysis.Issue{}, issues...)
	return nil
}

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(ctx context.Context, dsn string) (*PostgresStore, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		return nil, err
	}
	store := &PostgresStore{db: db}
	if err := store.ensureSchema(ctx); err != nil {
		return nil, err
	}
	return store, nil
}

func (p *PostgresStore) ensureSchema(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS pull_requests (
			id SERIAL PRIMARY KEY,
			repo TEXT NOT NULL,
			pr_number INTEGER NOT NULL,
			commit_sha TEXT NOT NULL,
			title TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE (repo, pr_number)
		);`,
		`CREATE TABLE IF NOT EXISTS analysis_results (
			id SERIAL PRIMARY KEY,
			pr_id INTEGER NOT NULL REFERENCES pull_requests(id) ON DELETE CASCADE,
			file TEXT NOT NULL,
			issue_type TEXT NOT NULL,
			severity TEXT NOT NULL,
			message TEXT NOT NULL,
			line INTEGER NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,
		`CREATE TABLE IF NOT EXISTS review_feedback (
			id SERIAL PRIMARY KEY,
			repo TEXT NOT NULL,
			rule_id TEXT NOT NULL,
			accepted BOOLEAN NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,
	}
	for _, stmt := range statements {
		if _, err := p.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func (p *PostgresStore) UpsertPullRequest(ctx context.Context, repo string, number int, sha string, title string, status string) (int64, error) {
	query := `
		INSERT INTO pull_requests (repo, pr_number, commit_sha, title, status)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (repo, pr_number)
		DO UPDATE SET commit_sha = EXCLUDED.commit_sha, title = EXCLUDED.title, status = EXCLUDED.status, updated_at = NOW()
		RETURNING id`
	var id int64
	if err := p.db.QueryRowContext(ctx, query, repo, number, sha, title, status).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func (p *PostgresStore) UpdatePullRequestStatus(ctx context.Context, id int64, status string) error {
	_, err := p.db.ExecContext(ctx, `UPDATE pull_requests SET status = $1, updated_at = NOW() WHERE id = $2`, status, id)
	return err
}

func (p *PostgresStore) SaveAnalysisResults(ctx context.Context, prID int64, issues []analysis.Issue) error {
	if len(issues) == 0 {
		return nil
	}
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO analysis_results (pr_id, file, issue_type, severity, message, line) VALUES ($1, $2, $3, $4, $5, $6)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, issue := range issues {
		if _, err := stmt.ExecContext(ctx, prID, issue.File, issue.RuleID, issue.Severity, issue.Message, issue.Line); err != nil {
			return err
		}
	}
	return tx.Commit()
}
