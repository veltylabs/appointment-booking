package appointmentbooking

import (
	"errors"

	"github.com/tinywasm/fmt"
	"github.com/tinywasm/orm"
	"github.com/tinywasm/unixid"
)

// Package-level sentinel errors
var (
	ErrNotFound = fmt.Err("record", "not", "found")
	ErrConflict = fmt.Err("optimistic", "concurrency", "conflict")
)

// Repository provides CRUD operations for all appointment-booking tables.
type Repository struct {
	db *orm.DB
}

// NewRepository creates a new Repository and auto-migrates all tables.
func NewRepository(db *orm.DB) (*Repository, error) {
	r := &Repository{db: db}
	tables := []orm.Model{
		&EmployeeServiceConfig{},
		&WorkCalendarConfig{},
		&WorkCalendarWeekly{},
		&WorkCalendarException{},
		&Reservation{},
	}
	for _, t := range tables {
		if err := db.CreateTable(t); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// ----------------------------------------------------------------------------
// Reservation
// ----------------------------------------------------------------------------

func (r *Repository) InsertReservation(res *Reservation) error {
	idHandler, err := unixid.NewUnixID()
	if err != nil {
		return err
	}
	if res.ID == "" {
		res.ID = idHandler.GetNewID()
	}
	res.Revision = 0
	return r.db.Create(res)
}

func (r *Repository) GetReservation(id string) (Reservation, error) {
	m := &Reservation{}
	qb := r.db.Query(m).Where(Reservation_.ID).Eq(id)
	got, err := ReadOneReservation(qb, m)
	if errors.Is(err, orm.ErrNotFound) {
		return Reservation{}, ErrNotFound
	}
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return Reservation{}, ErrNotFound
		}
		return Reservation{}, err
	}
	return *got, nil
}

func (r *Repository) GetReservationTx(tx *orm.DB, tenantID, id string) (Reservation, error) {
	m := &Reservation{}
	qb := tx.Query(m).Where(Reservation_.ID).Eq(id).Where(Reservation_.TenantID).Eq(tenantID)
	got, err := ReadOneReservation(qb, m)
	if errors.Is(err, orm.ErrNotFound) {
		return Reservation{}, ErrNotFound
	}
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return Reservation{}, ErrNotFound
		}
		return Reservation{}, err
	}
	return *got, nil
}

func (r *Repository) ListReservationsByStaff(tenantID, staffID string, from, to int64) ([]Reservation, error) {
	proxy := &Reservation{}
	qb := r.db.Query(proxy).
		Where(Reservation_.TenantID).Eq(tenantID).
		Where(Reservation_.StaffIDSnapshot).Eq(staffID).
		Where(Reservation_.ReservationDate).Gte(from).
		Where(Reservation_.ReservationDate).Lte(to)
	rows, err := ReadAllReservation(qb)
	if err != nil {
		return nil, err
	}
	out := make([]Reservation, len(rows))
	for i, row := range rows {
		out[i] = *row
	}
	return out, nil
}

func (r *Repository) ListReservationsByClient(tenantID, clientID string) ([]Reservation, error) {
	proxy := &Reservation{}
	qb := r.db.Query(proxy).
		Where(Reservation_.TenantID).Eq(tenantID).
		Where(Reservation_.ClientID).Eq(clientID)
	rows, err := ReadAllReservation(qb)
	if err != nil {
		return nil, err
	}
	out := make([]Reservation, len(rows))
	for i, row := range rows {
		out[i] = *row
	}
	return out, nil
}

func (r *Repository) UpdateReservationStatus(id, status, updatedBy string, updatedAt int64, expectedRevision int64) error {
	return r.db.Tx(func(tx *orm.DB) error {
		return r.UpdateReservationStatusTx(tx, id, status, updatedBy, updatedAt, expectedRevision)
	})
}

func (r *Repository) UpdateReservationStatusTx(tx *orm.DB, id, status, updatedBy string, updatedAt int64, expectedRevision int64) error {
	current := &Reservation{}
	qb := tx.Query(current).Where(Reservation_.ID).Eq(id)
	got, err := ReadOneReservation(qb, current)
	if errors.Is(err, orm.ErrNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if got.Revision != expectedRevision {
		return ErrConflict
	}
	got.Status = status
	got.UpdatedBy = updatedBy
	got.UpdatedAt = updatedAt
	got.Revision++
	return tx.Update(got, orm.Eq(Reservation_.ID, id))
}

// ----------------------------------------------------------------------------
// WorkCalendarException
// ----------------------------------------------------------------------------

func (r *Repository) InsertException(exc WorkCalendarException) error {
	idHandler, err := unixid.NewUnixID()
	if err != nil {
		return err
	}
	exc.ID = idHandler.GetNewID()
	return r.db.Create(&exc)
}

func (r *Repository) ListExceptions(tenantID, staffID string, from, to int64) ([]WorkCalendarException, error) {
	proxy := &WorkCalendarException{}
	qb := r.db.Query(proxy).
		Where(WorkCalendarException_.TenantID).Eq(tenantID).
		Where(WorkCalendarException_.StaffID).Eq(staffID).
		Where(WorkCalendarException_.SpecificDate).Gte(from).
		Where(WorkCalendarException_.SpecificDate).Lte(to)
	rows, err := ReadAllWorkCalendarException(qb)
	if err != nil {
		return nil, err
	}
	out := make([]WorkCalendarException, len(rows))
	for i, row := range rows {
		out[i] = *row
	}
	return out, nil
}

func (r *Repository) DeleteException(tenantID, id string) error {
	return r.db.Delete(&WorkCalendarException{}, orm.Eq(WorkCalendarException_.ID, id), orm.Eq(WorkCalendarException_.TenantID, tenantID))
}

// ----------------------------------------------------------------------------
// EmployeeServiceConfig
// ----------------------------------------------------------------------------

func (r *Repository) InsertEmployeeServiceConfig(cfg EmployeeServiceConfig) error {
	idHandler, err := unixid.NewUnixID()
	if err != nil {
		return err
	}
	cfg.ID = idHandler.GetNewID()
	return r.db.Create(&cfg)
}

func (r *Repository) GetEmployeeServiceConfig(id string) (EmployeeServiceConfig, error) {
	m := &EmployeeServiceConfig{}
	qb := r.db.Query(m).Where(EmployeeServiceConfig_.ID).Eq(id)
	got, err := ReadOneEmployeeServiceConfig(qb, m)
	if errors.Is(err, orm.ErrNotFound) {
		return EmployeeServiceConfig{}, ErrNotFound
	}
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return EmployeeServiceConfig{}, ErrNotFound
		}
		return EmployeeServiceConfig{}, err
	}
	return *got, nil
}

func (r *Repository) ListEmployeeServiceConfigByStaff(tenantID, staffID string) ([]EmployeeServiceConfig, error) {
	proxy := &EmployeeServiceConfig{}
	qb := r.db.Query(proxy).
		Where(EmployeeServiceConfig_.TenantID).Eq(tenantID).
		Where(EmployeeServiceConfig_.StaffID).Eq(staffID)
	rows, err := ReadAllEmployeeServiceConfig(qb)
	if err != nil {
		return nil, err
	}
	out := make([]EmployeeServiceConfig, len(rows))
	for i, row := range rows {
		out[i] = *row
	}
	return out, nil
}

func (r *Repository) UpdateEmployeeServiceConfig(cfg EmployeeServiceConfig) error {
	return r.db.Update(&cfg, orm.Eq(EmployeeServiceConfig_.ID, cfg.ID))
}

// ----------------------------------------------------------------------------
// WorkCalendarConfig
// ----------------------------------------------------------------------------

func (r *Repository) UpsertCalendarConfig(cfg WorkCalendarConfig) error {
	// Try to find existing record for this (tenantID, staffID)
	existing := &WorkCalendarConfig{}
	qb := r.db.Query(existing).
		Where(WorkCalendarConfig_.TenantID).Eq(cfg.TenantID).
		Where(WorkCalendarConfig_.StaffID).Eq(cfg.StaffID)
	got, err := ReadOneWorkCalendarConfig(qb, existing)
	if err != nil && !errors.Is(err, orm.ErrNotFound) && err.Error() != "sql: no rows in result set" {
		return err
	}
	if errors.Is(err, orm.ErrNotFound) || (err != nil && err.Error() == "sql: no rows in result set") {
		// Does not exist — create
		idHandler, err := unixid.NewUnixID()
		if err != nil {
			return err
		}
		cfg.ID = idHandler.GetNewID()
		return r.db.Create(&cfg)
	}
	// Exists — update in place (preserve original ID)
	cfg.ID = got.ID
	return r.db.Update(&cfg, orm.Eq(WorkCalendarConfig_.ID, cfg.ID))
}

func (r *Repository) GetCalendarConfig(tenantID, staffID string) (WorkCalendarConfig, error) {
	m := &WorkCalendarConfig{}
	qb := r.db.Query(m).
		Where(WorkCalendarConfig_.TenantID).Eq(tenantID).
		Where(WorkCalendarConfig_.StaffID).Eq(staffID)
	got, err := ReadOneWorkCalendarConfig(qb, m)
	if errors.Is(err, orm.ErrNotFound) {
		return WorkCalendarConfig{}, ErrNotFound
	}
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return WorkCalendarConfig{}, ErrNotFound
		}
		return WorkCalendarConfig{}, err
	}
	return *got, nil
}

// ----------------------------------------------------------------------------
// WorkCalendarWeekly
// ----------------------------------------------------------------------------

func (r *Repository) UpsertWeeklyCalendar(cal WorkCalendarWeekly) error {
	existing := &WorkCalendarWeekly{}
	qb := r.db.Query(existing).
		Where(WorkCalendarWeekly_.TenantID).Eq(cal.TenantID).
		Where(WorkCalendarWeekly_.StaffID).Eq(cal.StaffID).
		Where(WorkCalendarWeekly_.DayOfWeek).Eq(cal.DayOfWeek)
	got, err := ReadOneWorkCalendarWeekly(qb, existing)
	if err != nil && !errors.Is(err, orm.ErrNotFound) && err.Error() != "sql: no rows in result set" {
		return err
	}
	if errors.Is(err, orm.ErrNotFound) || (err != nil && err.Error() == "sql: no rows in result set") {
		idHandler, err := unixid.NewUnixID()
		if err != nil {
			return err
		}
		cal.ID = idHandler.GetNewID()
		return r.db.Create(&cal)
	}
	cal.ID = got.ID
	return r.db.Update(&cal, orm.Eq(WorkCalendarWeekly_.ID, cal.ID))
}

func (r *Repository) ListWeeklyCalendar(tenantID, staffID string) ([]WorkCalendarWeekly, error) {
	proxy := &WorkCalendarWeekly{}
	qb := r.db.Query(proxy).
		Where(WorkCalendarWeekly_.TenantID).Eq(tenantID).
		Where(WorkCalendarWeekly_.StaffID).Eq(staffID)
	rows, err := ReadAllWorkCalendarWeekly(qb)
	if err != nil {
		return nil, err
	}
	out := make([]WorkCalendarWeekly, len(rows))
	for i, row := range rows {
		out[i] = *row
	}
	return out, nil
}
