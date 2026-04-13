package appointmentbooking

import (
	"encoding/json"
	"errors"
	stdFmt "fmt"

	"github.com/tinywasm/context"
	"github.com/tinywasm/mcp"
)

// ReservationProvider implements mcp.ToolProvider for reservation tools.
type ReservationProvider struct{ svc SchedulingService }

func NewReservationProvider(svc SchedulingService) *ReservationProvider {
	return &ReservationProvider{svc: svc}
}

func (p *ReservationProvider) Tools() []mcp.Tool {
	return []mcp.Tool{
		{
			Name:        "create_reservation",
			Description: "Creates a new reservation.",
			Resource:    "reservation",
			Action:      'c',
			Execute:     p.createReservation,
		},
		{
			Name:        "get_reservation",
			Description: "Gets a reservation by ID.",
			Resource:    "reservation",
			Action:      'r',
			Execute:     p.getReservation,
		},
		{
			Name:        "list_reservations_by_staff",
			Description: "Lists reservations by staff ID and date range.",
			Resource:    "reservation",
			Action:      'r',
			Execute:     p.listReservationsByStaff,
		},
		{
			Name:        "list_reservations_by_client",
			Description: "Lists reservations by client ID.",
			Resource:    "reservation",
			Action:      'r',
			Execute:     p.listReservationsByClient,
		},
		{
			Name:        "change_reservation_status",
			Description: "Changes a reservation status via FSM event.",
			Resource:    "reservation",
			Action:      'u',
			Execute:     p.changeReservationStatus,
		},
		{
			Name:        "expire_pending_reservations",
			Description: "Expires unconfirmed pending reservations (called by scheduler).",
			Resource:    "reservation",
			Action:      'u',
			Execute:     p.expirePendingReservations,
		},
	}
}

func decodeArgs(req mcp.Request) (map[string]any, error) {
	var args map[string]any
	if err := json.Unmarshal([]byte(req.Params.Arguments), &args); err != nil {
		return nil, err
	}
	return args, nil
}

func errResult(msg string) *mcp.Result {
	return &mcp.Result{IsError: true, Content: msg}
}

func (p *ReservationProvider) createReservation(ctx *context.Context, req mcp.Request) (*mcp.Result, error) {
	args, err := decodeArgs(req)
	if err != nil {
		return errResult("invalid arguments"), nil
	}
	cmd := CreateReservationCmd{
		TenantID:                str(args, "tenant_id"),
		ClientID:                str(args, "client_id"),
		CreatorUserID:           str(args, "creator_user_id"),
		EmployeeServiceConfigID: str(args, "employee_service_config_id"),
		SlotStartUTC:            i64(args, "slot_start_utc"),
		Notes:                   str(args, "notes"),
		RescheduledFromID:       str(args, "rescheduled_from_id"),
	}
	res, err := p.svc.CreateReservation(ToStd(ctx), cmd)
	if err != nil {
		if errors.Is(err, ErrSlotTaken) {
			return errResult("The selected time slot is already taken"), nil
		}
		return errResult(err.Error()), nil
	}
	b, _ := json.Marshal(res)
	return mcp.Text(string(b)), nil
}

func (p *ReservationProvider) getReservation(ctx *context.Context, req mcp.Request) (*mcp.Result, error) {
	args, err := decodeArgs(req)
	if err != nil {
		return errResult("invalid arguments"), nil
	}
	res, err := p.svc.GetReservation(ToStd(ctx), str(args, "tenant_id"), str(args, "id"))
	if err != nil {
		return errResult(err.Error()), nil
	}
	b, _ := json.Marshal(res)
	return mcp.Text(string(b)), nil
}

func (p *ReservationProvider) listReservationsByStaff(ctx *context.Context, req mcp.Request) (*mcp.Result, error) {
	args, err := decodeArgs(req)
	if err != nil {
		return errResult("invalid arguments"), nil
	}
	res, err := p.svc.ListReservationsByStaff(ToStd(ctx), str(args, "tenant_id"), str(args, "staff_id"), i64(args, "from"), i64(args, "to"))
	if err != nil {
		return errResult(err.Error()), nil
	}
	b, _ := json.Marshal(res)
	return mcp.Text(string(b)), nil
}

func (p *ReservationProvider) listReservationsByClient(ctx *context.Context, req mcp.Request) (*mcp.Result, error) {
	args, err := decodeArgs(req)
	if err != nil {
		return errResult("invalid arguments"), nil
	}
	res, err := p.svc.ListReservationsByClient(ToStd(ctx), str(args, "tenant_id"), str(args, "client_id"))
	if err != nil {
		return errResult(err.Error()), nil
	}
	b, _ := json.Marshal(res)
	return mcp.Text(string(b)), nil
}

func (p *ReservationProvider) changeReservationStatus(ctx *context.Context, req mcp.Request) (*mcp.Result, error) {
	args, err := decodeArgs(req)
	if err != nil {
		return errResult("invalid arguments"), nil
	}
	cmd := ChangeStatusCmd{
		TenantID:  str(args, "tenant_id"),
		ID:        str(args, "id"),
		Event:     str(args, "event"),
		ActorID:   str(args, "actor_id"),
		PaymentID: str(args, "payment_id"),
		Revision:  int(i64(args, "revision")),
	}
	if err := p.svc.ChangeReservationStatus(ToStd(ctx), cmd); err != nil {
		return errResult(err.Error()), nil
	}
	return mcp.Text("Status updated successfully"), nil
}

func (p *ReservationProvider) expirePendingReservations(ctx *context.Context, req mcp.Request) (*mcp.Result, error) {
	args, err := decodeArgs(req)
	if err != nil {
		return errResult("invalid arguments"), nil
	}
	count, err := p.svc.ExpirePendingReservations(ToStd(ctx), str(args, "tenant_id"), i64(args, "before"))
	if err != nil {
		return errResult(err.Error()), nil
	}
	return mcp.Text(stdFmt.Sprintf("Expired %d reservations", count)), nil
}

// CalendarProvider implements mcp.ToolProvider for calendar tools.
type CalendarProvider struct{ svc SchedulingService }

func NewCalendarProvider(svc SchedulingService) *CalendarProvider {
	return &CalendarProvider{svc: svc}
}

func (p *CalendarProvider) Tools() []mcp.Tool {
	return []mcp.Tool{
		{
			Name:        "upsert_calendar_config",
			Description: "Sets IANA timezone for a staff member. Must be called before upsert_weekly_calendar.",
			Resource:    "calendar",
			Action:      'u',
			Execute:     p.upsertCalendarConfig,
		},
		{
			Name:        "upsert_weekly_calendar",
			Description: "Sets weekly schedule for a staff member.",
			Resource:    "calendar",
			Action:      'u',
			Execute:     p.upsertWeeklyCalendar,
		},
		{
			Name:        "add_calendar_exception",
			Description: "Adds a calendar exception for a specific date.",
			Resource:    "calendar",
			Action:      'c',
			Execute:     p.addCalendarException,
		},
		{
			Name:        "remove_calendar_exception",
			Description: "Removes a calendar exception.",
			Resource:    "calendar",
			Action:      'd',
			Execute:     p.removeCalendarException,
		},
		{
			Name:        "list_availability",
			Description: "Lists available time slots for a staff member.",
			Resource:    "calendar",
			Action:      'r',
			Execute:     p.listAvailability,
		},
	}
}

func (p *CalendarProvider) upsertCalendarConfig(ctx *context.Context, req mcp.Request) (*mcp.Result, error) {
	args, err := decodeArgs(req)
	if err != nil {
		return errResult("invalid arguments"), nil
	}
	cfg := WorkCalendarConfig{
		TenantID: str(args, "tenant_id"),
		StaffID:  str(args, "staff_id"),
		Timezone: str(args, "timezone"),
		IsActive: boolVal(args, "is_active"),
	}
	if err := p.svc.UpsertCalendarConfig(ToStd(ctx), cfg); err != nil {
		return errResult(err.Error()), nil
	}
	return mcp.Text("Calendar config upserted successfully"), nil
}

func (p *CalendarProvider) upsertWeeklyCalendar(ctx *context.Context, req mcp.Request) (*mcp.Result, error) {
	args, err := decodeArgs(req)
	if err != nil {
		return errResult("invalid arguments"), nil
	}
	cal := WorkCalendarWeekly{
		TenantID:    str(args, "tenant_id"),
		StaffID:     str(args, "staff_id"),
		DayOfWeek:   i64(args, "day_of_week"),
		WorkStart:   i64(args, "work_start"),
		WorkFinish:  i64(args, "work_finish"),
		BreakStart:  i64(args, "break_start"),
		BreakFinish: i64(args, "break_finish"),
		IsActive:    boolVal(args, "is_active"),
	}
	if err := p.svc.UpsertWeeklyCalendar(ToStd(ctx), cal); err != nil {
		if errors.Is(err, ErrCalendarConfigNotFound) {
			return errResult("Set the staff timezone first using upsert_calendar_config"), nil
		}
		return errResult(err.Error()), nil
	}
	return mcp.Text("Weekly calendar upserted successfully"), nil
}

func (p *CalendarProvider) addCalendarException(ctx *context.Context, req mcp.Request) (*mcp.Result, error) {
	args, err := decodeArgs(req)
	if err != nil {
		return errResult("invalid arguments"), nil
	}
	exc := WorkCalendarException{
		TenantID:      str(args, "tenant_id"),
		StaffID:       str(args, "staff_id"),
		SpecificDate:  i64(args, "specific_date"),
		ExceptionType: str(args, "exception_type"),
		StartTime:     i64(args, "start_time"),
		EndTime:       i64(args, "end_time"),
		Notes:         str(args, "notes"),
	}
	if err := p.svc.AddException(ToStd(ctx), exc); err != nil {
		return errResult(err.Error()), nil
	}
	return mcp.Text("Calendar exception added successfully"), nil
}

func (p *CalendarProvider) removeCalendarException(ctx *context.Context, req mcp.Request) (*mcp.Result, error) {
	args, err := decodeArgs(req)
	if err != nil {
		return errResult("invalid arguments"), nil
	}
	if err := p.svc.RemoveException(ToStd(ctx), str(args, "tenant_id"), str(args, "exception_id")); err != nil {
		return errResult(err.Error()), nil
	}
	return mcp.Text("Calendar exception removed successfully"), nil
}

func (p *CalendarProvider) listAvailability(ctx *context.Context, req mcp.Request) (*mcp.Result, error) {
	args, err := decodeArgs(req)
	if err != nil {
		return errResult("invalid arguments"), nil
	}
	slots, err := p.svc.ListAvailability(ToStd(ctx), str(args, "tenant_id"), str(args, "staff_id"), str(args, "config_id"), i64(args, "from"), i64(args, "to"))
	if err != nil {
		if errors.Is(err, ErrCalendarConfigNotFound) {
			return errResult("Set the staff timezone first using upsert_calendar_config"), nil
		}
		return errResult(err.Error()), nil
	}
	b, err := json.Marshal(slots)
	if err != nil {
		return errResult(err.Error()), nil
	}
	return mcp.Text(string(b)), nil
}

// arg helpers
func str(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return v
}

func i64(args map[string]any, key string) int64 {
	switch v := args[key].(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case int:
		return int64(v)
	}
	return 0
}

func boolVal(args map[string]any, key string) bool {
	v, _ := args[key].(bool)
	return v
}
