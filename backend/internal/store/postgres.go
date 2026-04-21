package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/Trin9/SynapseFlow/backend/internal/audit"
	"github.com/Trin9/SynapseFlow/backend/pkg/models"
)

type PostgresStores struct {
	DB         *sql.DB
	DAGs       *PostgresDAGStore
	Executions *PostgresExecutionStore
	Audits     *PostgresAuditStore
	memories   *PostgresExperienceStore
}

func OpenPostgres(ctx context.Context, dsn string, maxOpen, maxIdle int, maxIdleTime, maxLifetime time.Duration) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxIdleTime(maxIdleTime)
	db.SetConnMaxLifetime(maxLifetime)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func NewPostgresStores(db *sql.DB) *PostgresStores {
	return &PostgresStores{
		DB:         db,
		DAGs:       &PostgresDAGStore{db: db},
		Executions: &PostgresExecutionStore{db: db},
		Audits:     &PostgresAuditStore{db: db},
		memories:   &PostgresExperienceStore{db: db},
	}
}

func (s *PostgresStores) MemoryStore() *PostgresExperienceStore {
	return s.memories
}

type PostgresDAGStore struct{ db *sql.DB }

func (s *PostgresDAGStore) Create(ctx context.Context, dag *models.DAGConfig) error {
	nodesJSON, err := json.Marshal(dag.Nodes)
	if err != nil {
		return err
	}
	edgesJSON, err := json.Marshal(dag.Edges)
	if err != nil {
		return err
	}
	metadataJSON, err := json.Marshal(dag.Metadata)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO dag_configs (id, name, description, metadata, nodes, edges, created_at, updated_at)
		VALUES ($1, $2, $3, $4::jsonb, $5::jsonb, $6::jsonb, $7, $8)
	`, dag.ID, dag.Name, dag.Description, metadataJSON, nodesJSON, edgesJSON, dag.CreatedAt.UTC(), dag.UpdatedAt.UTC())
	return err
}

func (s *PostgresDAGStore) Update(ctx context.Context, dag *models.DAGConfig) error {
	nodesJSON, err := json.Marshal(dag.Nodes)
	if err != nil {
		return err
	}
	edgesJSON, err := json.Marshal(dag.Edges)
	if err != nil {
		return err
	}
	metadataJSON, err := json.Marshal(dag.Metadata)
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE dag_configs
		SET name = $2, description = $3, metadata = $4::jsonb, nodes = $5::jsonb, edges = $6::jsonb, updated_at = $7
		WHERE id = $1
	`, dag.ID, dag.Name, dag.Description, metadataJSON, nodesJSON, edgesJSON, dag.UpdatedAt.UTC())
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresDAGStore) Delete(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM dag_configs WHERE id = $1`, id)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresDAGStore) Get(ctx context.Context, id string) (*models.DAGConfig, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, description, metadata, nodes, edges, created_at, updated_at
		FROM dag_configs WHERE id = $1
	`, id)
	return scanDAG(row)
}

func (s *PostgresDAGStore) List(ctx context.Context) ([]*models.DAGConfig, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, description, metadata, nodes, edges, created_at, updated_at
		FROM dag_configs ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*models.DAGConfig, 0)
	for rows.Next() {
		dag, err := scanDAG(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, dag)
	}
	return out, rows.Err()
}

type PostgresExecutionStore struct{ db *sql.DB }

func (s *PostgresExecutionStore) Create(ctx context.Context, exec *models.Execution) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO executions (id, dag_id, dag_name, status, error, started_at, ended_at, duration_ms, state, loop_counts)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, $10::jsonb)
	`, exec.ID, exec.DAGID, exec.DAGName, exec.Status, exec.Error, exec.StartedAt.UTC(), nullableTime(exec.EndedAt), exec.Duration.Milliseconds(), mustJSON(executionStateSnapshot(exec)), mustJSON(executionLoopCountsSnapshot(exec)))
	return err
}

func (s *PostgresExecutionStore) Update(ctx context.Context, exec *models.Execution) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE executions
		SET dag_id = $2, dag_name = $3, status = $4, error = $5, started_at = $6, ended_at = $7, duration_ms = $8, state = $9::jsonb, loop_counts = $10::jsonb
		WHERE id = $1
	`, exec.ID, exec.DAGID, exec.DAGName, exec.Status, exec.Error, exec.StartedAt.UTC(), nullableTime(exec.EndedAt), exec.Duration.Milliseconds(), mustJSON(executionStateSnapshot(exec)), mustJSON(executionLoopCountsSnapshot(exec)))
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresExecutionStore) Get(ctx context.Context, id string) (*models.Execution, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, dag_id, dag_name, status, error, started_at, ended_at, duration_ms, state, loop_counts
		FROM executions WHERE id = $1
	`, id)
	exec, err := scanExecution(row)
	if err != nil {
		return nil, err
	}
	results, err := s.ListNodeResults(ctx, id)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	exec.Results = results
	return exec, nil
}

func (s *PostgresExecutionStore) List(ctx context.Context) ([]*models.Execution, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, dag_id, dag_name, status, error, started_at, ended_at, duration_ms, state, loop_counts
		FROM executions ORDER BY started_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*models.Execution, 0)
	for rows.Next() {
		exec, err := scanExecution(rows)
		if err != nil {
			return nil, err
		}
		results, err := s.ListNodeResults(ctx, exec.ID)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return nil, err
		}
		exec.Results = results
		out = append(out, exec)
	}
	return out, rows.Err()
}

func (s *PostgresExecutionStore) SaveNodeResults(ctx context.Context, executionID string, results []models.NodeResult) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM node_executions WHERE execution_id = $1`, executionID); err != nil {
		return err
	}
	for idx, result := range results {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO node_executions (
				execution_id, ordinal, node_id, node_name, node_type, status, output, error, duration_ms, tokens_in, tokens_out
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		`, executionID, idx, result.NodeID, result.NodeName, result.NodeType, result.Status, result.Output, result.Error, result.Duration.Milliseconds(), result.TokensIn, result.TokensOut); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *PostgresExecutionStore) ListNodeResults(ctx context.Context, executionID string) ([]models.NodeResult, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT node_id, node_name, node_type, status, output, error, duration_ms, tokens_in, tokens_out
		FROM node_executions WHERE execution_id = $1 ORDER BY ordinal ASC
	`, executionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	results := make([]models.NodeResult, 0)
	for rows.Next() {
		var result models.NodeResult
		var durationMS int64
		if err := rows.Scan(&result.NodeID, &result.NodeName, &result.NodeType, &result.Status, &result.Output, &result.Error, &durationMS, &result.TokensIn, &result.TokensOut); err != nil {
			return nil, err
		}
		result.Duration = time.Duration(durationMS) * time.Millisecond
		results = append(results, result)
	}
	if len(results) == 0 {
		return []models.NodeResult{}, rows.Err()
	}
	return results, rows.Err()
}

func (s *PostgresExecutionStore) SaveCheckpoint(ctx context.Context, checkpoint *models.ExecutionCheckpoint) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO execution_checkpoints (execution_id, dag_id, state, loop_counts, updated_at)
		VALUES ($1, $2, $3::jsonb, $4::jsonb, $5)
		ON CONFLICT (execution_id)
		DO UPDATE SET dag_id = EXCLUDED.dag_id, state = EXCLUDED.state, loop_counts = EXCLUDED.loop_counts, updated_at = EXCLUDED.updated_at
	`, checkpoint.ExecutionID, checkpoint.DAGID, mustJSON(checkpoint.State), mustJSON(checkpoint.LoopCounts), checkpoint.UpdatedAt.UTC())
	return err
}

func (s *PostgresExecutionStore) GetCheckpoint(ctx context.Context, executionID string) (*models.ExecutionCheckpoint, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT execution_id, dag_id, state, loop_counts, updated_at
		FROM execution_checkpoints WHERE execution_id = $1
	`, executionID)
	var checkpoint models.ExecutionCheckpoint
	var stateJSON []byte
	var loopCountsJSON []byte
	if err := row.Scan(&checkpoint.ExecutionID, &checkpoint.DAGID, &stateJSON, &loopCountsJSON, &checkpoint.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if err := json.Unmarshal(stateJSON, &checkpoint.State); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(loopCountsJSON, &checkpoint.LoopCounts); err != nil {
		return nil, err
	}
	return &checkpoint, nil
}

type PostgresAuditStore struct{ db *sql.DB }

func (s *PostgresAuditStore) Record(ctx context.Context, entry audit.Entry) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO audit_logs (time, actor, role, action, resource, resource_id, result, details)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, entry.Time.UTC(), entry.Actor, entry.Role, entry.Action, entry.Resource, entry.ResourceID, entry.Result, entry.Details)
	return err
}

func (s *PostgresAuditStore) List(ctx context.Context) ([]audit.Entry, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT time, actor, role, action, resource, resource_id, result, details
		FROM audit_logs ORDER BY time DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := make([]audit.Entry, 0)
	for rows.Next() {
		var entry audit.Entry
		if err := rows.Scan(&entry.Time, &entry.Actor, &entry.Role, &entry.Action, &entry.Resource, &entry.ResourceID, &entry.Result, &entry.Details); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

type PostgresExperienceStore struct{ db *sql.DB }

func (s *PostgresExperienceStore) Save(ctx context.Context, exp *models.Experience) error {
	tagsJSON, err := json.Marshal(exp.Tags)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO experiences (id, alert_type, service_name, tags, symptom, root_cause, action_taken, summary, document, created_at, updated_at)
		VALUES ($1, $2, $3, $4::jsonb, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (id)
		DO UPDATE SET alert_type = EXCLUDED.alert_type, service_name = EXCLUDED.service_name, tags = EXCLUDED.tags,
		  symptom = EXCLUDED.symptom, root_cause = EXCLUDED.root_cause, action_taken = EXCLUDED.action_taken,
		  summary = EXCLUDED.summary, document = EXCLUDED.document, updated_at = EXCLUDED.updated_at
	`, exp.ID, exp.AlertType, exp.ServiceName, tagsJSON, exp.Symptom, exp.RootCause, exp.ActionTaken, exp.Summary, exp.Document, exp.CreatedAt.UTC(), exp.UpdatedAt.UTC())
	return err
}

func (s *PostgresExperienceStore) List(ctx context.Context) ([]models.Experience, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, alert_type, service_name, tags, symptom, root_cause, action_taken, summary, document, created_at, updated_at
		FROM experiences ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanExperiences(rows)
}

func (s *PostgresExperienceStore) Search(ctx context.Context, query SearchQuery) ([]models.Experience, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, alert_type, service_name, tags, symptom, root_cause, action_taken, summary, document, created_at, updated_at
		FROM experiences
		WHERE ($1 = '' OR alert_type = $1)
		  AND ($2 = '' OR service_name = $2)
		ORDER BY created_at DESC
	`, query.AlertType, query.ServiceName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	experiences, err := scanExperiences(rows)
	if err != nil {
		return nil, err
	}
	if len(experiences) == 0 {
		return []models.Experience{}, nil
	}
	inMemory := NewInMemoryExperienceAdapter(experiences)
	return inMemory.Search(ctx, query)
}

type scanner interface {
	Scan(dest ...interface{}) error
}

func scanDAG(row scanner) (*models.DAGConfig, error) {
	var dag models.DAGConfig
	var metadataJSON []byte
	var nodesJSON []byte
	var edgesJSON []byte
	if err := row.Scan(&dag.ID, &dag.Name, &dag.Description, &metadataJSON, &nodesJSON, &edgesJSON, &dag.CreatedAt, &dag.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &dag.Metadata); err != nil {
			return nil, err
		}
	}
	if err := json.Unmarshal(nodesJSON, &dag.Nodes); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(edgesJSON, &dag.Edges); err != nil {
		return nil, err
	}
	return &dag, nil
}

func scanExecution(row scanner) (*models.Execution, error) {
	var exec models.Execution
	var durationMS int64
	var stateJSON []byte
	var loopCountsJSON []byte
	var endedAt sql.NullTime
	if err := row.Scan(&exec.ID, &exec.DAGID, &exec.DAGName, &exec.Status, &exec.Error, &exec.StartedAt, &endedAt, &durationMS, &stateJSON, &loopCountsJSON); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if endedAt.Valid {
		exec.EndedAt = &endedAt.Time
	}
	exec.Duration = time.Duration(durationMS) * time.Millisecond
	var state map[string]interface{}
	var loopCounts map[string]int
	if len(stateJSON) > 0 {
		if err := json.Unmarshal(stateJSON, &state); err != nil {
			return nil, err
		}
	}
	if len(loopCountsJSON) > 0 {
		if err := json.Unmarshal(loopCountsJSON, &loopCounts); err != nil {
			return nil, err
		}
	}
	exec.State = models.NewGlobalStateFromSnapshot(state, loopCounts)
	return &exec, nil
}

func scanExperiences(rows *sql.Rows) ([]models.Experience, error) {
	experiences := make([]models.Experience, 0)
	for rows.Next() {
		var exp models.Experience
		var tagsJSON []byte
		if err := rows.Scan(&exp.ID, &exp.AlertType, &exp.ServiceName, &tagsJSON, &exp.Symptom, &exp.RootCause, &exp.ActionTaken, &exp.Summary, &exp.Document, &exp.CreatedAt, &exp.UpdatedAt); err != nil {
			return nil, err
		}
		if len(tagsJSON) > 0 {
			if err := json.Unmarshal(tagsJSON, &exp.Tags); err != nil {
				return nil, err
			}
		}
		experiences = append(experiences, exp)
	}
	return experiences, rows.Err()
}

func nullableTime(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return t.UTC()
}

func mustJSON(v interface{}) []byte {
	if v == nil {
		return []byte(`{}`)
	}
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("marshal json: %v", err))
	}
	return b
}

type MemoryExperienceAdapter struct {
	items []models.Experience
}

func NewInMemoryExperienceAdapter(items []models.Experience) *MemoryExperienceAdapter {
	copyItems := append([]models.Experience(nil), items...)
	return &MemoryExperienceAdapter{items: copyItems}
}

func (a *MemoryExperienceAdapter) Search(_ context.Context, query SearchQuery) ([]models.Experience, error) {
	textTokens := tokenize(query.Text)
	topK := query.TopK
	if topK <= 0 {
		topK = 3
	}
	type scored struct {
		exp   models.Experience
		score float64
	}
	results := make([]scored, 0, len(a.items))
	for _, exp := range a.items {
		docTokens := tokenize(exp.Document + "\n" + exp.Summary + "\n" + exp.RootCause + "\n" + exp.ActionTaken)
		score := overlapScore(textTokens, docTokens)
		if score <= 0 {
			continue
		}
		exp.Score = score
		results = append(results, scored{exp: exp, score: score})
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].score == results[j].score {
			return results[i].exp.CreatedAt.After(results[j].exp.CreatedAt)
		}
		return results[i].score > results[j].score
	})
	if len(results) > topK {
		results = results[:topK]
	}
	out := make([]models.Experience, 0, len(results))
	for _, item := range results {
		out = append(out, item.exp)
	}
	return out, nil
}

func executionStateSnapshot(exec *models.Execution) map[string]interface{} {
	if exec == nil || exec.State == nil {
		return map[string]interface{}{}
	}
	return exec.State.Snapshot()
}

func executionLoopCountsSnapshot(exec *models.Execution) map[string]int {
	if exec == nil || exec.State == nil {
		return map[string]int{}
	}
	return exec.State.LoopCountsSnapshot()
}

func tokenize(text string) map[string]struct{} {
	parts := strings.Fields(strings.ToLower(strings.NewReplacer(",", " ", ".", " ", ":", " ", ";", " ", "\n", " ", "\t", " ").Replace(text)))
	set := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		set[trimmed] = struct{}{}
	}
	return set
}

func overlapScore(a, b map[string]struct{}) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	matched := 0
	for token := range a {
		if _, ok := b[token]; ok {
			matched++
		}
	}
	if matched == 0 {
		return 0
	}
	denominator := len(a)
	if len(b) < denominator {
		denominator = len(b)
	}
	return float64(matched) / float64(denominator)
}
