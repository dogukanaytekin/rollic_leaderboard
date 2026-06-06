package worker

import (
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var boardColumns = []string{"id", "created_at", "interval_seconds"}

// now sabitleyelim — tüm testlerde deterministik
var fixedNow = time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)

// 7 günlük interval
const weekSeconds = int64(604800)

// --- runCleanupAt ---

func TestRunCleanupAt_NoScheduledBoards(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery(`SELECT id, created_at, interval_seconds FROM boards`).
		WillReturnRows(sqlmock.NewRows(boardColumns))

	err = runCleanupAt(db, fixedNow)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRunCleanupAt_BoardInFirstPeriod_NothingDeleted(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Board 3 gün önce oluşturuldu, 7 günlük interval — hâlâ 1. periyotta
	// periodStart = createdAt, yani scored_at < createdAt olan hiç kayıt yok
	createdAt := fixedNow.Add(-3 * 24 * time.Hour)

	mock.ExpectQuery(`SELECT id, created_at, interval_seconds FROM boards`).
		WillReturnRows(sqlmock.NewRows(boardColumns).
			AddRow(1, createdAt, weekSeconds))

	// periodStart = createdAt (dump=0), DELETE 0 satır etkiler → loop biter
	mock.ExpectExec(`DELETE FROM scores`).
		WithArgs(int64(1), createdAt, batchSize).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = runCleanupAt(db, fixedNow)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRunCleanupAt_BoardInSecondPeriod_OldScoresDeleted(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Board 10 gün önce oluşturuldu, 7 günlük interval — 2. periyotta
	// dump = floor(10/7) = 1
	// periodStart = createdAt + 7 gün
	createdAt := fixedNow.Add(-10 * 24 * time.Hour)
	periodStart := createdAt.Add(time.Duration(weekSeconds) * time.Second)

	mock.ExpectQuery(`SELECT id, created_at, interval_seconds FROM boards`).
		WillReturnRows(sqlmock.NewRows(boardColumns).
			AddRow(1, createdAt, weekSeconds))

	// 500 eski satır var, batchSize'dan az → tek iteration
	mock.ExpectExec(`DELETE FROM scores`).
		WithArgs(int64(1), periodStart, batchSize).
		WillReturnResult(sqlmock.NewResult(0, 500))

	err = runCleanupAt(db, fixedNow)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRunCleanupAt_ThirdPeriod_CorrectPeriodStart(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Board 20 gün önce oluşturuldu → dump = floor(20/7) = 2 → 3. periyot
	// periodStart = createdAt + 2*7 gün
	createdAt := fixedNow.Add(-20 * 24 * time.Hour)
	periodStart := createdAt.Add(2 * time.Duration(weekSeconds) * time.Second)

	mock.ExpectQuery(`SELECT id, created_at, interval_seconds FROM boards`).
		WillReturnRows(sqlmock.NewRows(boardColumns).
			AddRow(1, createdAt, weekSeconds))

	mock.ExpectExec(`DELETE FROM scores`).
		WithArgs(int64(1), periodStart, batchSize).
		WillReturnResult(sqlmock.NewResult(0, 100))

	err = runCleanupAt(db, fixedNow)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRunCleanupAt_BatchDeletion_LessThanBatchSize(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	createdAt := fixedNow.Add(-10 * 24 * time.Hour)
	periodStart := createdAt.Add(time.Duration(weekSeconds) * time.Second)

	mock.ExpectQuery(`SELECT id, created_at, interval_seconds FROM boards`).
		WillReturnRows(sqlmock.NewRows(boardColumns).
			AddRow(1, createdAt, weekSeconds))

	// 9999 satır → batchSize'dan az → tek iteration, loop biter
	mock.ExpectExec(`DELETE FROM scores`).
		WithArgs(int64(1), periodStart, batchSize).
		WillReturnResult(sqlmock.NewResult(0, 9999))

	err = runCleanupAt(db, fixedNow)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRunCleanupAt_BatchDeletion_ExactlyBatchSize(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	createdAt := fixedNow.Add(-10 * 24 * time.Hour)
	periodStart := createdAt.Add(time.Duration(weekSeconds) * time.Second)

	mock.ExpectQuery(`SELECT id, created_at, interval_seconds FROM boards`).
		WillReturnRows(sqlmock.NewRows(boardColumns).
			AddRow(1, createdAt, weekSeconds))

	// Tam batchSize → daha fazla olabilir diye tekrar dener
	mock.ExpectExec(`DELETE FROM scores`).
		WithArgs(int64(1), periodStart, batchSize).
		WillReturnResult(sqlmock.NewResult(0, int64(batchSize)))

	// 2. iteration → 0 satır → loop biter
	mock.ExpectExec(`DELETE FROM scores`).
		WithArgs(int64(1), periodStart, batchSize).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = runCleanupAt(db, fixedNow)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRunCleanupAt_BatchDeletion_MultipleBatches(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	createdAt := fixedNow.Add(-10 * 24 * time.Hour)
	periodStart := createdAt.Add(time.Duration(weekSeconds) * time.Second)

	mock.ExpectQuery(`SELECT id, created_at, interval_seconds FROM boards`).
		WillReturnRows(sqlmock.NewRows(boardColumns).
			AddRow(1, createdAt, weekSeconds))

	// 25000 satır → 3 batch: 10000 + 10000 + 5000
	mock.ExpectExec(`DELETE FROM scores`).
		WithArgs(int64(1), periodStart, batchSize).
		WillReturnResult(sqlmock.NewResult(0, int64(batchSize)))
	mock.ExpectExec(`DELETE FROM scores`).
		WithArgs(int64(1), periodStart, batchSize).
		WillReturnResult(sqlmock.NewResult(0, int64(batchSize)))
	mock.ExpectExec(`DELETE FROM scores`).
		WithArgs(int64(1), periodStart, batchSize).
		WillReturnResult(sqlmock.NewResult(0, 5000))

	err = runCleanupAt(db, fixedNow)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRunCleanupAt_MultipleBoards_EachCleanedIndependently(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	createdAt1 := fixedNow.Add(-10 * 24 * time.Hour)
	periodStart1 := createdAt1.Add(time.Duration(weekSeconds) * time.Second)

	createdAt2 := fixedNow.Add(-15 * 24 * time.Hour)
	periodStart2 := createdAt2.Add(2 * time.Duration(weekSeconds) * time.Second)

	mock.ExpectQuery(`SELECT id, created_at, interval_seconds FROM boards`).
		WillReturnRows(sqlmock.NewRows(boardColumns).
			AddRow(1, createdAt1, weekSeconds).
			AddRow(2, createdAt2, weekSeconds))

	mock.ExpectExec(`DELETE FROM scores`).
		WithArgs(int64(1), periodStart1, batchSize).
		WillReturnResult(sqlmock.NewResult(0, 300))

	mock.ExpectExec(`DELETE FROM scores`).
		WithArgs(int64(2), periodStart2, batchSize).
		WillReturnResult(sqlmock.NewResult(0, 800))

	err = runCleanupAt(db, fixedNow)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRunCleanupAt_FetchError_ReturnsError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery(`SELECT id, created_at, interval_seconds FROM boards`).
		WillReturnError(errors.New("db connection lost"))

	err = runCleanupAt(db, fixedNow)
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRunCleanupAt_DeleteError_LogsAndContinues(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	createdAt1 := fixedNow.Add(-10 * 24 * time.Hour)
	periodStart1 := createdAt1.Add(time.Duration(weekSeconds) * time.Second)

	createdAt2 := fixedNow.Add(-10 * 24 * time.Hour)
	periodStart2 := createdAt2.Add(time.Duration(weekSeconds) * time.Second)

	mock.ExpectQuery(`SELECT id, created_at, interval_seconds FROM boards`).
		WillReturnRows(sqlmock.NewRows(boardColumns).
			AddRow(1, createdAt1, weekSeconds).
			AddRow(2, createdAt2, weekSeconds))

	// board 1 hata verir
	mock.ExpectExec(`DELETE FROM scores`).
		WithArgs(int64(1), periodStart1, batchSize).
		WillReturnError(errors.New("timeout"))

	// board 2 yine de temizlenir
	mock.ExpectExec(`DELETE FROM scores`).
		WithArgs(int64(2), periodStart2, batchSize).
		WillReturnResult(sqlmock.NewResult(0, 50))

	// RunCleanup nil döner — board bazlı hata tüm cleanup'ı durdurmaz
	err = runCleanupAt(db, fixedNow)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRunCleanupAt_PeriodBoundary_ScoreAtPeriodStartNotDeleted(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Board tam 7 gün önce oluşturuldu → dump=1 → periodStart = createdAt + 7 gün = fixedNow
	// scored_at < periodStart → fixedNow'dan önceki skorlar silinir
	// scored_at = periodStart → SİLİNMEZ (< değil, <=)
	createdAt := fixedNow.Add(-7 * 24 * time.Hour)
	periodStart := createdAt.Add(time.Duration(weekSeconds) * time.Second) // = fixedNow

	mock.ExpectQuery(`SELECT id, created_at, interval_seconds FROM boards`).
		WillReturnRows(sqlmock.NewRows(boardColumns).
			AddRow(1, createdAt, weekSeconds))

	mock.ExpectExec(`DELETE FROM scores`).
		WithArgs(int64(1), periodStart, batchSize).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = runCleanupAt(db, fixedNow)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRunCleanupAt_FetchRowsErr_ReturnsError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Satır döner ama rows.Next() sonrası rows.Err() hata verir
	mock.ExpectQuery(`SELECT id, created_at, interval_seconds FROM boards`).
		WillReturnRows(sqlmock.NewRows(boardColumns).
			AddRow(1, fixedNow.Add(-10*24*time.Hour), weekSeconds).
			RowError(0, errors.New("connection reset")))

	err = runCleanupAt(db, fixedNow)
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRunCleanupAt_RowsAffectedError_LogsAndContinues(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	createdAt := fixedNow.Add(-10 * 24 * time.Hour)
	periodStart := createdAt.Add(time.Duration(weekSeconds) * time.Second)

	mock.ExpectQuery(`SELECT id, created_at, interval_seconds FROM boards`).
		WillReturnRows(sqlmock.NewRows(boardColumns).
			AddRow(1, createdAt, weekSeconds).
			AddRow(2, createdAt, weekSeconds))

	// board 1: RowsAffected() hata verir
	mock.ExpectExec(`DELETE FROM scores`).
		WithArgs(int64(1), periodStart, batchSize).
		WillReturnResult(sqlmock.NewErrorResult(errors.New("driver does not support RowsAffected")))

	// board 2 yine de temizlenir
	mock.ExpectExec(`DELETE FROM scores`).
		WithArgs(int64(2), periodStart, batchSize).
		WillReturnResult(sqlmock.NewResult(0, 100))

	err = runCleanupAt(db, fixedNow)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRunCleanupAt_DifferentIntervals(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Board 1: günlük interval (86400s), 3 gün önce → 3. periyot
	dailyInterval := int64(86400)
	createdAt1 := fixedNow.Add(-3 * 24 * time.Hour)
	periodStart1 := createdAt1.Add(3 * time.Duration(dailyInterval) * time.Second) // = fixedNow

	// Board 2: aylık interval (2592000s), 45 gün önce → 2. periyot
	monthlyInterval := int64(2592000)
	createdAt2 := fixedNow.Add(-45 * 24 * time.Hour)
	periodStart2 := createdAt2.Add(time.Duration(monthlyInterval) * time.Second)

	mock.ExpectQuery(`SELECT id, created_at, interval_seconds FROM boards`).
		WillReturnRows(sqlmock.NewRows(boardColumns).
			AddRow(1, createdAt1, dailyInterval).
			AddRow(2, createdAt2, monthlyInterval))

	mock.ExpectExec(`DELETE FROM scores`).
		WithArgs(int64(1), periodStart1, batchSize).
		WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectExec(`DELETE FROM scores`).
		WithArgs(int64(2), periodStart2, batchSize).
		WillReturnResult(sqlmock.NewResult(0, 1200))

	err = runCleanupAt(db, fixedNow)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
