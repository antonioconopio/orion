package handlers

import (
	"github.com/jackc/pgx/v5"

	"orion-api/internal/models"
)

// dagSummarySelect reads a DAG plus stats from its recent runs. Callers append a WHERE or ORDER BY.
// "Finished" means success or failed; stats use the last 20 finished runs, the sparkline the last 10.
const dagSummarySelect = `
	SELECT d.id, d.name, d.description, d.schedule, d.owner, d.is_active, d.python_version, d.created_at, d.updated_at,
	       lr.id, lr.number, lr.status, lr.started_at, lr.finished_at,
	       st.success_rate, st.avg_duration,
	       COALESCE(rd.durations, '{}')
	FROM dags d
	LEFT JOIN LATERAL (
		SELECT id, number, status, started_at, finished_at
		FROM runs WHERE dag_id = d.id ORDER BY created_at DESC LIMIT 1
	) lr ON true
	LEFT JOIN LATERAL (
		SELECT avg(CASE WHEN status = 'success' THEN 1.0 ELSE 0.0 END)::float8 AS success_rate,
		       avg(EXTRACT(EPOCH FROM finished_at - started_at))::float8 AS avg_duration
		FROM (
			SELECT status, started_at, finished_at FROM runs
			WHERE dag_id = d.id AND status IN ('success', 'failed')
			ORDER BY created_at DESC LIMIT 20
		) f
	) st ON true
	LEFT JOIN LATERAL (
		SELECT array_agg(dur ORDER BY created_at) AS durations
		FROM (
			SELECT created_at, EXTRACT(EPOCH FROM finished_at - started_at)::float8 AS dur
			FROM runs
			WHERE dag_id = d.id AND status IN ('success', 'failed')
			  AND started_at IS NOT NULL AND finished_at IS NOT NULL
			ORDER BY created_at DESC LIMIT 10
		) x
	) rd ON true`

// scanDAGSummary reads the columns of dagSummarySelect from a pgx.Row or pgx.Rows.
func scanDAGSummary(row pgx.Row, d *models.DAGSummary) error {
	var lr models.RunBrief
	var lrID *string // NULL when the DAG has never run
	var lrNumber *int
	var lrStatus *string
	err := row.Scan(&d.ID, &d.Name, &d.Description, &d.Schedule, &d.Owner, &d.IsActive, &d.PythonVersion, &d.CreatedAt, &d.UpdatedAt,
		&lrID, &lrNumber, &lrStatus, &lr.StartedAt, &lr.FinishedAt,
		&d.SuccessRate, &d.AvgDurationSeconds, &d.RecentDurations)
	if err != nil {
		return err
	}
	if lrID != nil {
		if err := lr.ID.UnmarshalText([]byte(*lrID)); err != nil {
			return err
		}
		lr.Number, lr.Status = *lrNumber, *lrStatus
		d.LastRun = &lr
	}
	return nil
}
