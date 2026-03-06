package appointmentbooking

import (
	"context"
	"errors"
	"time"

	"github.com/tinywasm/fmt"
	"github.com/tinywasm/orm"
	"github.com/tinywasm/unixid"
	tinytime "github.com/tinywasm/time"
)

var (
	ErrCalendarConfigNotFound = fmt.Err("calendar", "config", "not", "found")
	ErrSlotTaken              = fmt.Err("slot", "taken")
)

// Domain events emitted by this module.
const (
	EventReservationCreated     = "appointment.reservation.created"
	EventReservationConfirmed   = "appointment.reservation.confirmed"
	EventReservationCancelled   = "appointment.reservation.cancelled"
	EventReservationCompleted   = "appointment.reservation.completed"
	EventReservationNoShow      = "appointment.reservation.no_show"
	EventReservationExpired     = "appointment.reservation.expired"
	EventReservationRescheduled = "appointment.reservation.rescheduled"
)

// StaffReader verifies a staff member exists and belongs to the tenant.
type StaffReader interface {
	StaffExists(tenantID, staffID string) (bool, error)
}

// CatalogReader verifies a service exists and belongs to the tenant.
type CatalogReader interface {
	ServiceExists(tenantID, serviceID string) (bool, error)
}

// DirectoryReader verifies a client exists and belongs to the tenant.
type DirectoryReader interface {
	ClientExists(tenantID, clientID string) (bool, error)
}

// EventPublisher delivers domain events to other modules or infrastructure.
type EventPublisher interface {
	Publish(ctx context.Context, event string, payload any) error
}

type SchedulingService interface {
	// Calendar management
	UpsertCalendarConfig(ctx context.Context, cfg WorkCalendarConfig) error
	UpsertWeeklyCalendar(ctx context.Context, cal WorkCalendarWeekly) error
	AddException(ctx context.Context, exc WorkCalendarException) error
	RemoveException(ctx context.Context, tenantID, exceptionID string) error

	// Availability
	ListAvailability(ctx context.Context, tenantID, staffID, configID string, from, to int64) ([]TimeSlot, error)

	// Reservations
	CreateReservation(ctx context.Context, cmd CreateReservationCmd) (Reservation, error)
	GetReservation(ctx context.Context, tenantID, id string) (Reservation, error)
	ListReservationsByStaff(ctx context.Context, tenantID, staffID string, from, to int64) ([]Reservation, error)
	ListReservationsByClient(ctx context.Context, tenantID, clientID string) ([]Reservation, error)
	ChangeReservationStatus(ctx context.Context, cmd ChangeStatusCmd) error
	ExpirePendingReservations(ctx context.Context, tenantID string, before int64) (int, error)
}

type CreateReservationCmd struct {
	TenantID                string
	ClientID                string
	CreatorUserID           string
	EmployeeServiceConfigID string
	SlotStartUTC            int64
	Notes                   string
	RescheduledFromID       string
}

type ChangeStatusCmd struct {
	TenantID  string
	ID        string
	Event     string
	ActorID   string
	PaymentID string
	Revision  int
}

type schedulingService struct {
	db        *orm.DB
	repo      *Repository
	staff     StaffReader
	catalog   CatalogReader
	directory DirectoryReader
	pub       EventPublisher
}

type Deps struct {
	Staff     StaffReader
	Catalog   CatalogReader
	Directory DirectoryReader
	Publisher EventPublisher
}

func New(db *orm.DB, deps Deps) (SchedulingService, error) {
	repo, err := NewRepository(db)
	if err != nil {
		return nil, err
	}

	return &schedulingService{
		db:        db,
		repo:      repo,
		staff:     deps.Staff,
		catalog:   deps.Catalog,
		directory: deps.Directory,
		pub:       deps.Publisher,
	}, nil
}

func (s *schedulingService) UpsertCalendarConfig(ctx context.Context, cfg WorkCalendarConfig) error {
	return s.repo.UpsertCalendarConfig(cfg)
}

func (s *schedulingService) UpsertWeeklyCalendar(ctx context.Context, cal WorkCalendarWeekly) error {
	// Must check if CalendarConfig exists first
	_, err := s.repo.GetCalendarConfig(cal.TenantID, cal.StaffID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrCalendarConfigNotFound
		}
		return err
	}

	return s.repo.UpsertWeeklyCalendar(cal)
}

func (s *schedulingService) AddException(ctx context.Context, exc WorkCalendarException) error {
	return s.repo.InsertException(exc)
}

func (s *schedulingService) RemoveException(ctx context.Context, tenantID, exceptionID string) error {
	return s.repo.DeleteException(tenantID, exceptionID)
}

// LocalIntToUnixUTC interprets localInt as minutes from midnight on the given date (UTC midnight) in the given tz.
func LocalIntToUnixUTC(date int64, localInt int, tz string) int64 {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	// date is UTC midnight. We want the same calendar day in the local timezone.
	utcDate := time.Unix(date, 0).UTC()

	hour := localInt / 60
	minute := localInt % 60

	localTime := time.Date(utcDate.Year(), utcDate.Month(), utcDate.Day(), hour, minute, 0, 0, loc)
	return localTime.Unix()
}

func (s *schedulingService) ListAvailability(ctx context.Context, tenantID, staffID, configID string, from, to int64) ([]TimeSlot, error) {
	// 1. Load WorkCalendarConfig
	cfg, err := s.repo.GetCalendarConfig(tenantID, staffID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrCalendarConfigNotFound
		}
		return nil, err
	}
	if !cfg.IsActive {
		return []TimeSlot{}, nil
	}

	// 2. Load WorkCalendarWeekly
	weeklies, err := s.repo.ListWeeklyCalendar(tenantID, staffID)
	if err != nil {
		return nil, err
	}
	activeWeeklies := make(map[int]WorkCalendarWeekly)
	for _, w := range weeklies {
		if w.IsActive {
			activeWeeklies[int(w.DayOfWeek)] = w
		}
	}

	// 3. Load WorkCalendarException
	exceptions, err := s.repo.ListExceptions(tenantID, staffID, from, to)
	if err != nil {
		return nil, err
	}
	exceptionsByDate := make(map[int64][]WorkCalendarException)
	for _, e := range exceptions {
		exceptionsByDate[e.SpecificDate] = append(exceptionsByDate[e.SpecificDate], e)
	}

	// 4. Load existing Reservations
	reservations, err := s.repo.ListReservationsByStaff(tenantID, staffID, from, to)
	if err != nil {
		return nil, err
	}
	activeReservations := []Reservation{}
	for _, r := range reservations {
		if r.Status != StatusCancelled && r.Status != StatusRescheduled && r.Status != StatusExpired {
			activeReservations = append(activeReservations, r)
		}
	}

	// 5. Load EmployeeServiceConfig
	empSvcCfg, err := s.repo.GetEmployeeServiceConfig(configID)
	if err != nil {
		return nil, err
	}
	if !empSvcCfg.IsActive || empSvcCfg.TenantID != tenantID {
		return []TimeSlot{}, nil
	}

	durationMin := int(empSvcCfg.DurationMin)
	bufferMin := int(empSvcCfg.BufferMin)

	slots := []TimeSlot{}

	// 6. For each day D in [from, to] (assuming from and to are midnight UTC timestamps)
	// We increment by 1 day (86400 seconds)
	for d := from; d <= to; d += 86400 {
		tDate := time.Unix(d, 0).UTC()
		dow := int(tDate.Weekday())

		weekly, hasWeekly := activeWeeklies[dow]
		if !hasWeekly {
			continue // skip day
		}

		workStartUTC := LocalIntToUnixUTC(d, int(weekly.WorkStart), cfg.Timezone)
		workFinishUTC := LocalIntToUnixUTC(d, int(weekly.WorkFinish), cfg.Timezone)
		breakStartUTC := LocalIntToUnixUTC(d, int(weekly.BreakStart), cfg.Timezone)
		breakFinishUTC := LocalIntToUnixUTC(d, int(weekly.BreakFinish), cfg.Timezone)

		hasBreak := weekly.BreakStart > 0 || weekly.BreakFinish > 0

		// Apply exceptions
		dayExceptions := exceptionsByDate[d]
		isHoliday := false

		// Priority: HOLIDAY > SPECIAL_HOURS > BLOCKED
		var specialHours *WorkCalendarException
		var blockedExcs []WorkCalendarException

		for _, e := range dayExceptions {
			eCopy := e
			if e.ExceptionType == "HOLIDAY" {
				isHoliday = true
			} else if e.ExceptionType == "SPECIAL_HOURS" {
				if specialHours == nil {
					specialHours = &eCopy
				}
			} else if e.ExceptionType == "BLOCKED" {
				blockedExcs = append(blockedExcs, e)
			}
		}

		if isHoliday {
			continue
		}

		if specialHours != nil {
			workStartUTC = LocalIntToUnixUTC(d, int(specialHours.StartTime), cfg.Timezone)
			workFinishUTC = LocalIntToUnixUTC(d, int(specialHours.EndTime), cfg.Timezone)
			hasBreak = false // break interval removed
		}

		// Generate slots
		curr := workStartUTC
		for {
			end := curr + int64(durationMin*60)
			endWithBuffer := end + int64(bufferMin*60)

			if endWithBuffer > workFinishUTC {
				break
			}

			// Check break
			if hasBreak {
				if !(end <= breakStartUTC || curr >= breakFinishUTC) {
					// skip to the end of the break to allow slots after break
					curr = breakFinishUTC
					continue
				}
			}

			// Check blocked exceptions
			isBlocked := false
			var blockedEnd int64
			for _, b := range blockedExcs {
				bStart := LocalIntToUnixUTC(d, int(b.StartTime), cfg.Timezone)
				bEnd := LocalIntToUnixUTC(d, int(b.EndTime), cfg.Timezone)
				// Overlap check
				if curr < bEnd && endWithBuffer > bStart {
					isBlocked = true
					blockedEnd = bEnd
					break
				}
			}
			if isBlocked {
				curr = blockedEnd // advance past block
				continue
			}

			// Check existing reservations
			hasOverlap := false
			var resEnd int64
			for _, r := range activeReservations {
				rStart := r.ReservationTime
				rEnd := rStart + int64(r.DurationMinSnapshot*60)
				// Overlap check
				if curr < rEnd && endWithBuffer > rStart {
					hasOverlap = true
					resEnd = rEnd
					break
				}
			}

			if hasOverlap {
				curr = resEnd
			} else {
				slots = append(slots, TimeSlot{StartUTC: curr, EndUTC: end})
				curr += int64(durationMin * 60)
			}
		}
	}

	return slots, nil
}

func (s *schedulingService) CreateReservation(ctx context.Context, cmd CreateReservationCmd) (Reservation, error) {
	// 1. Load EmployeeServiceConfig
	empSvcCfg, err := s.repo.GetEmployeeServiceConfig(cmd.EmployeeServiceConfigID)
	if err != nil {
		return Reservation{}, err
	}
	if !empSvcCfg.IsActive || empSvcCfg.TenantID != cmd.TenantID {
		return Reservation{}, ErrNotFound
	}

	// 2. Validate client
	clientExists, err := s.directory.ClientExists(cmd.TenantID, cmd.ClientID)
	if err != nil {
		return Reservation{}, err
	}
	if !clientExists {
		return Reservation{}, fmt.Err("client", "not", "found")
	}

	// 3. Validate staff
	staffExists, err := s.staff.StaffExists(cmd.TenantID, empSvcCfg.StaffID)
	if err != nil {
		return Reservation{}, err
	}
	if !staffExists {
		return Reservation{}, fmt.Err("staff", "not", "found")
	}

	// 4. Validate service
	serviceExists, err := s.catalog.ServiceExists(cmd.TenantID, empSvcCfg.ServiceID)
	if err != nil {
		return Reservation{}, err
	}
	if !serviceExists {
		return Reservation{}, fmt.Err("service", "not", "found")
	}

	// 5. Check availability
	// Get availability for the target day (midnight UTC)
	utcDate := time.Unix(cmd.SlotStartUTC, 0).UTC()
	targetDay := time.Date(utcDate.Year(), utcDate.Month(), utcDate.Day(), 0, 0, 0, 0, time.UTC).Unix()

	// Broaden the search by one day on each side to account for timezone boundary differences
	fromDay := targetDay - 86400
	toDay := targetDay + 86400

	slots, err := s.ListAvailability(ctx, cmd.TenantID, empSvcCfg.StaffID, empSvcCfg.ID, fromDay, toDay)
	if err != nil {
		return Reservation{}, err
	}

	isAvailable := false
	for _, slot := range slots {
		if slot.StartUTC == cmd.SlotStartUTC {
			isAvailable = true
			break
		}
	}
	if !isAvailable {
		return Reservation{}, ErrSlotTaken
	}

	var newReservation Reservation
	var originalReservation *Reservation

	err = s.db.Tx(func(tx *orm.DB) error {
		now := tinytime.Now()

		newReservation = Reservation{
			TenantID:                cmd.TenantID,
			ClientID:                cmd.ClientID,
			CreatorUserID:           cmd.CreatorUserID,
			EmployeeServiceConfigID: cmd.EmployeeServiceConfigID,
			StaffIDSnapshot:         empSvcCfg.StaffID,
			ServiceIDSnapshot:       empSvcCfg.ServiceID,
			DurationMinSnapshot:     empSvcCfg.DurationMin,
			PriceSnapshot:           empSvcCfg.PriceOverride,
			CurrencySnapshot:        "CLP", // default
			ReservationDate:         targetDay,
			ReservationTime:         cmd.SlotStartUTC,
			LocalStringDate:         tinytime.FormatDate(cmd.SlotStartUTC * 1000000000),
			LocalStringTime:         tinytime.FormatTime(cmd.SlotStartUTC * 1000000000),
			Status:                  StatusPending,
			RescheduledFromID:       cmd.RescheduledFromID,
			Notes:                   cmd.Notes,
			UpdatedAt:               now,
			UpdatedBy:               cmd.CreatorUserID, // Using CreatorUserID as the ActorID at creation
			Revision:                0,
		}

		if cmd.RescheduledFromID != "" {
			orig, err := s.repo.GetReservationTx(tx, cmd.TenantID, cmd.RescheduledFromID)
			if err != nil {
				return err
			}
			originalReservation = &orig

			_, err = Transition(orig.Status, EventReschedule)
			if err != nil {
				return err
			}

			err = s.repo.UpdateReservationStatusTx(tx, orig.ID, StatusRescheduled, cmd.CreatorUserID, now, orig.Revision)
			if err != nil {
				return err
			}
		}

		idHandler, err := unixid.NewUnixID()
		if err != nil {
			return err
		}
		// ensure a unique ID by using the generated nanosecond one properly
		newReservation.ID = idHandler.GetNewID()

		// Do an in-tx insert instead of repo.InsertReservation which uses db.Create
		err = tx.Create(&newReservation)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return Reservation{}, err
	}

	if s.pub != nil {
		s.pub.Publish(ctx, EventReservationCreated, newReservation)
		if originalReservation != nil {
			s.pub.Publish(ctx, EventReservationRescheduled, *originalReservation)
		}
	}

	return newReservation, nil
}

func (s *schedulingService) GetReservation(ctx context.Context, tenantID, id string) (Reservation, error) {
	res, err := s.repo.GetReservation(id)
	if err != nil {
		return Reservation{}, err
	}
	if res.TenantID != tenantID {
		return Reservation{}, ErrNotFound
	}
	return res, nil
}

func (s *schedulingService) ListReservationsByStaff(ctx context.Context, tenantID, staffID string, from, to int64) ([]Reservation, error) {
	return s.repo.ListReservationsByStaff(tenantID, staffID, from, to)
}

func (s *schedulingService) ListReservationsByClient(ctx context.Context, tenantID, clientID string) ([]Reservation, error) {
	return s.repo.ListReservationsByClient(tenantID, clientID)
}

func (s *schedulingService) ChangeReservationStatus(ctx context.Context, cmd ChangeStatusCmd) error {
	current, err := s.repo.GetReservation(cmd.ID)
	if err != nil {
		return err
	}
	if current.TenantID != cmd.TenantID {
		return ErrNotFound
	}

	nextState, err := Transition(current.Status, cmd.Event)
	if err != nil {
		return err
	}

	err = s.db.Tx(func(tx *orm.DB) error {
		now := tinytime.Now()

		if cmd.Event == EventConfirm && cmd.PaymentID != "" {
			got := &Reservation{}
			qb := tx.Query(got).Where(Reservation_.ID).Eq(cmd.ID)
			gotRes, err := ReadOneReservation(qb, got)
			if err != nil {
				return err
			}
			if gotRes.Revision != int64(cmd.Revision) {
				return ErrConflict
			}
			gotRes.Status = nextState
			gotRes.UpdatedBy = cmd.ActorID
			gotRes.UpdatedAt = now
			gotRes.PaymentID = cmd.PaymentID
			gotRes.Revision++
			return tx.Update(gotRes, orm.Eq(Reservation_.ID, gotRes.ID)) // Update does a full update based on PK
		}

		return s.repo.UpdateReservationStatusTx(tx, cmd.ID, nextState, cmd.ActorID, now, int64(cmd.Revision))
	})

	if err != nil {
		return err
	}

	var domainEvent string
	switch cmd.Event {
	case EventConfirm:
		domainEvent = EventReservationConfirmed
	case EventCancel:
		domainEvent = EventReservationCancelled
	case EventComplete:
		domainEvent = EventReservationCompleted
	case EventNoShow:
		domainEvent = EventReservationNoShow
	case EventExpire:
		domainEvent = EventReservationExpired
	}

	if s.pub != nil && domainEvent != "" {
		// fetch updated
		updated, _ := s.repo.GetReservation(cmd.ID)
		s.pub.Publish(ctx, domainEvent, updated)
	}

	return nil
}

func (s *schedulingService) ExpirePendingReservations(ctx context.Context, tenantID string, before int64) (int, error) {
	proxy := &Reservation{}
	qb := s.db.Query(proxy).
		Where(Reservation_.TenantID).Eq(tenantID).
		Where(Reservation_.Status).Eq(StatusPending).
		Where(Reservation_.ReservationTime).Lt(before)

	rows, err := ReadAllReservation(qb)
	if err != nil {
		if errors.Is(err, orm.ErrNotFound) || err.Error() == "sql: no rows in result set" {
			return 0, nil
		}
		return 0, err
	}

	expiredCount := 0
	for _, row := range rows {
		err := s.ChangeReservationStatus(ctx, ChangeStatusCmd{
			TenantID: tenantID,
			ID:       row.ID,
			Event:    EventExpire,
			ActorID:  "system",
			Revision: int(row.Revision),
		})
		if err != nil {
			return expiredCount, err
		}
		expiredCount++
	}

	return expiredCount, nil
}
