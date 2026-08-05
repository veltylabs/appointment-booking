package appointmentbooking

import (
	"github.com/tinywasm/events"
	"github.com/tinywasm/fmt"
	"github.com/tinywasm/model"
	"github.com/tinywasm/orm"
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
	StaffExists(tenantId, staffId string) (bool, error)
}

// CatalogReader verifies a service exists and belongs to the tenant.
type CatalogReader interface {
	ServiceExists(tenantId, serviceId string) (bool, error)
}

// DirectoryReader verifies a client exists and belongs to the tenant.
type DirectoryReader interface {
	ClientExists(tenantId, clientId string) (bool, error)
}

type SchedulingService interface {
	// Calendar management
	UpsertCalendarConfig(cfg WorkCalendarConfig) error
	UpsertWeeklyCalendar(cal WorkCalendarWeekly) error
	AddException(exc WorkCalendarException) error
	RemoveException(tenantId, exceptionId string) error

	// Availability
	ListAvailability(tenantId, staffId, configId string, from, to int64) ([]TimeSlot, error)

	// Reservations
	CreateReservation(cmd CreateReservationCmd) (Reservation, error)
	GetReservation(tenantId, id string) (Reservation, error)
	ListReservationsByStaff(tenantId, staffId string, from, to int64) ([]Reservation, error)
	ListReservationsByClient(tenantId, clientId string) ([]Reservation, error)
	ChangeReservationStatus(cmd ChangeStatusCmd) error
	ExpirePendingReservations(tenantId string, before int64) (int, error)
}

type CreateReservationCmd struct {
	TenantId                string
	ClientId                string
	CreatorUserId           string
	EmployeeServiceConfigId string
	SlotStartUtc            int64
	Notes                   string
	RescheduledFromId       string
}

type ChangeStatusCmd struct {
	TenantId  string
	Id        string
	Event     string
	ActorId   string
	PaymentId string
	Revision  int
}

type Module struct {
	db        *orm.DB
	repo      *Repository
	ids       model.IDGenerator
	staff     StaffReader
	catalog   CatalogReader
	directory DirectoryReader
	pub       events.Publisher
}

type Deps struct {
	Staff     StaffReader
	Catalog   CatalogReader
	Directory DirectoryReader
	IDs       model.IDGenerator // requerido
	Publisher events.Publisher  // opcional — nil desactiva
}

func New(db *orm.DB, deps Deps) (*Module, error) {
	if deps.IDs == nil {
		return nil, fmt.Err("appointment_booking: Deps.IDs is required")
	}
	repo, err := NewRepository(db, deps.IDs)
	if err != nil {
		return nil, err
	}

	return &Module{
		db:        db,
		repo:      repo,
		ids:       deps.IDs,
		staff:     deps.Staff,
		catalog:   deps.Catalog,
		directory: deps.Directory,
		pub:       deps.Publisher,
	}, nil
}

var _ SchedulingService = (*Module)(nil)

func (m *Module) UpsertCalendarConfig(cfg WorkCalendarConfig) error {
	return m.repo.UpsertCalendarConfig(cfg)
}

func (m *Module) UpsertWeeklyCalendar(cal WorkCalendarWeekly) error {
	// Must check if CalendarConfig exists first
	_, err := m.repo.GetCalendarConfig(cal.TenantId, cal.StaffId)
	if err != nil {
		if err == ErrNotFound {
			return ErrCalendarConfigNotFound
		}
		return err
	}

	return m.repo.UpsertWeeklyCalendar(cal)
}

func (m *Module) AddException(exc WorkCalendarException) error {
	return m.repo.InsertException(exc)
}

func (m *Module) RemoveException(tenantId, exceptionId string) error {
	return m.repo.DeleteException(tenantId, exceptionId)
}

// LocalIntToUnixUTC interprets localInt as minutes from midnight on the given date (UTC midnight) in the given tz.
func LocalIntToUnixUTC(date int64, localInt int, tz string) int64 {
	return tinytime.LocalMinutesToUnixUTC(date, localInt, tz)
}

func (m *Module) ListAvailability(tenantId, staffId, configId string, from, to int64) ([]TimeSlot, error) {
	// 1. Load WorkCalendarConfig
	cfg, err := m.repo.GetCalendarConfig(tenantId, staffId)
	if err != nil {
		if err == ErrNotFound {
			return nil, ErrCalendarConfigNotFound
		}
		return nil, err
	}
	if !cfg.IsActive {
		return []TimeSlot{}, nil
	}

	// 2. Load WorkCalendarWeekly
	weeklies, err := m.repo.ListWeeklyCalendar(tenantId, staffId)
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
	exceptions, err := m.repo.ListExceptions(tenantId, staffId, from, to)
	if err != nil {
		return nil, err
	}
	exceptionsByDate := make(map[int64][]WorkCalendarException)
	for _, e := range exceptions {
		exceptionsByDate[e.SpecificDate] = append(exceptionsByDate[e.SpecificDate], e)
	}

	// 4. Load existing Reservations
	reservations, err := m.repo.ListReservationsByStaff(tenantId, staffId, from, to)
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
	empSvcCfg, err := m.repo.GetEmployeeServiceConfig(configId)
	if err != nil {
		return nil, err
	}
	if !empSvcCfg.IsActive || empSvcCfg.TenantId != tenantId {
		return []TimeSlot{}, nil
	}

	durationMin := int(empSvcCfg.DurationMin)
	bufferMin := int(empSvcCfg.BufferMin)

	slots := []TimeSlot{}

	// 6. For each day D in [from, to] (assuming from and to are midnight UTC timestamps)
	// We increment by 1 day (86400 seconds)
	for d := from; d <= to; d += 86400 {
		dow := tinytime.Weekday(d)

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
				slots = append(slots, TimeSlot{StartUtc: curr, EndUtc: end})
				curr += int64(durationMin * 60)
			}
		}
	}

	return slots, nil
}

func (m *Module) CreateReservation(cmd CreateReservationCmd) (Reservation, error) {
	// 1. Load EmployeeServiceConfig
	empSvcCfg, err := m.repo.GetEmployeeServiceConfig(cmd.EmployeeServiceConfigId)
	if err != nil {
		return Reservation{}, err
	}
	if !empSvcCfg.IsActive || empSvcCfg.TenantId != cmd.TenantId {
		return Reservation{}, ErrNotFound
	}

	// 2. Validate client
	clientExists, err := m.directory.ClientExists(cmd.TenantId, cmd.ClientId)
	if err != nil {
		return Reservation{}, err
	}
	if !clientExists {
		return Reservation{}, fmt.Err("client", "not", "found")
	}

	// 3. Validate staff
	staffExists, err := m.staff.StaffExists(cmd.TenantId, empSvcCfg.StaffId)
	if err != nil {
		return Reservation{}, err
	}
	if !staffExists {
		return Reservation{}, fmt.Err("staff", "not", "found")
	}

	// 4. Validate service
	serviceExists, err := m.catalog.ServiceExists(cmd.TenantId, empSvcCfg.ServiceId)
	if err != nil {
		return Reservation{}, err
	}
	if !serviceExists {
		return Reservation{}, fmt.Err("service", "not", "found")
	}

	// 5. Check availability
	// Get availability for the target day (midnight UTC)
	targetDay := tinytime.MidnightUTC(cmd.SlotStartUtc)

	// Broaden the search by one day on each side to account for timezone boundary differences
	fromDay := targetDay - 86400
	toDay := targetDay + 86400

	slots, err := m.ListAvailability(cmd.TenantId, empSvcCfg.StaffId, empSvcCfg.Id, fromDay, toDay)
	if err != nil {
		return Reservation{}, err
	}

	isAvailable := false
	for _, slot := range slots {
		if slot.StartUtc == cmd.SlotStartUtc {
			isAvailable = true
			break
		}
	}
	if !isAvailable {
		return Reservation{}, ErrSlotTaken
	}

	var newReservation Reservation
	var originalReservation *Reservation

	err = m.db.Tx(func(tx *orm.DB) error {
		now := tinytime.Now()

		newReservation = Reservation{
			TenantId:                cmd.TenantId,
			ClientId:                cmd.ClientId,
			CreatorUserId:           cmd.CreatorUserId,
			EmployeeServiceConfigId: cmd.EmployeeServiceConfigId,
			StaffIdsnapshot:         empSvcCfg.StaffId,
			ServiceIdsnapshot:       empSvcCfg.ServiceId,
			DurationMinSnapshot:     empSvcCfg.DurationMin,
			PriceSnapshot:           empSvcCfg.PriceOverride,
			CurrencySnapshot:        "CLP", // default
			ReservationDate:         targetDay,
			ReservationTime:         cmd.SlotStartUtc,
			LocalStringDate:         tinytime.FormatDate(cmd.SlotStartUtc * 1000000000),
			LocalStringTime:         tinytime.FormatTime(cmd.SlotStartUtc * 1000000000),
			Status:                  StatusPending,
			RescheduledFromId:       cmd.RescheduledFromId,
			Notes:                   cmd.Notes,
			UpdatedAt:               now,
			UpdatedBy:               cmd.CreatorUserId, // Using CreatorUserId as the ActorID at creation
			Revision:                0,
		}

		if cmd.RescheduledFromId != "" {
			orig, err := m.repo.GetReservationTx(tx, cmd.TenantId, cmd.RescheduledFromId)
			if err != nil {
				return err
			}
			originalReservation = &orig

			_, err = Transition(orig.Status, EventReschedule)
			if err != nil {
				return err
			}

			err = m.repo.UpdateReservationStatusTx(tx, orig.Id, StatusRescheduled, cmd.CreatorUserId, now, orig.Revision)
			if err != nil {
				return err
			}
		}

		newReservation.Id = m.ids.NewID()

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

	if m.pub != nil {
		m.pub.Publish(events.Event{Topic: EventReservationCreated, Payload: &newReservation})
		if originalReservation != nil {
			m.pub.Publish(events.Event{Topic: EventReservationRescheduled, Payload: originalReservation})
		}
	}

	return newReservation, nil
}

func (m *Module) GetReservation(tenantId, id string) (Reservation, error) {
	res, err := m.repo.GetReservation(id)
	if err != nil {
		return Reservation{}, err
	}
	if res.TenantId != tenantId {
		return Reservation{}, ErrNotFound
	}
	return res, nil
}

func (m *Module) ListReservationsByStaff(tenantId, staffId string, from, to int64) ([]Reservation, error) {
	return m.repo.ListReservationsByStaff(tenantId, staffId, from, to)
}

func (m *Module) ListReservationsByClient(tenantId, clientId string) ([]Reservation, error) {
	return m.repo.ListReservationsByClient(tenantId, clientId)
}

func (m *Module) ChangeReservationStatus(cmd ChangeStatusCmd) error {
	current, err := m.repo.GetReservation(cmd.Id)
	if err != nil {
		return err
	}
	if current.TenantId != cmd.TenantId {
		return ErrNotFound
	}

	nextState, err := Transition(current.Status, cmd.Event)
	if err != nil {
		return err
	}

	err = m.db.Tx(func(tx *orm.DB) error {
		now := tinytime.Now()

		if cmd.Event == EventConfirm && cmd.PaymentId != "" {
			got := &Reservation{}
			qb := tx.Query(got).Where(Reservation_.Id).Eq(cmd.Id)
			gotRes, err := ReadOneReservation(qb, got)
			if err != nil {
				return err
			}
			if gotRes.Revision != int64(cmd.Revision) {
				return ErrConflict
			}
			gotRes.Status = nextState
			gotRes.UpdatedBy = cmd.ActorId
			gotRes.UpdatedAt = now
			gotRes.PaymentId = cmd.PaymentId
			gotRes.Revision++
			return tx.Update(gotRes, orm.Eq(Reservation_.Id, gotRes.Id), orm.Eq(Reservation_.TenantId, gotRes.TenantId))
		}

		return m.repo.UpdateReservationStatusTx(tx, cmd.Id, nextState, cmd.ActorId, now, int64(cmd.Revision))
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

	if m.pub != nil && domainEvent != "" {
		// fetch updated
		updated, _ := m.repo.GetReservation(cmd.Id)
		m.pub.Publish(events.Event{Topic: domainEvent, Payload: &updated})
	}

	return nil
}

func (m *Module) ExpirePendingReservations(tenantId string, before int64) (int, error) {
	proxy := &Reservation{}
	qb := m.db.Query(proxy).
		Where(Reservation_.TenantId).Eq(tenantId).
		Where(Reservation_.Status).Eq(StatusPending).
		Where(Reservation_.ReservationTime).Lt(before)

	rows, err := ReadAllReservation(qb)
	if err != nil {
		if err == orm.ErrNotFound {
			return 0, nil
		}
		return 0, err
	}

	expiredCount := 0
	for _, row := range rows {
		err := m.ChangeReservationStatus(ChangeStatusCmd{
			TenantId: tenantId,
			Id:       row.Id,
			Event:    EventExpire,
			ActorId:  "system",
			Revision: int(row.Revision),
		})
		if err != nil {
			return expiredCount, err
		}
		expiredCount++
	}

	return expiredCount, nil
}
