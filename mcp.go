package appointmentbooking

import (
	"github.com/tinywasm/context"
	"github.com/tinywasm/fmt"
	"github.com/tinywasm/json"
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

func errResult(msg string) *mcp.Result {
	return &mcp.Result{IsError: true, Content: msg}
}

func (p *ReservationProvider) createReservation(ctx *context.Context, req mcp.Request) (*mcp.Result, error) {
	var args createReservationArgs
	if err := req.Bind(&args); err != nil {
		return errResult("invalid arguments"), nil
	}
	cmd := CreateReservationCmd{
		TenantID:                args.TenantID,
		ClientID:                args.ClientID,
		CreatorUserID:           args.CreatorUserID,
		EmployeeServiceConfigID: args.EmployeeServiceConfigID,
		SlotStartUTC:            args.SlotStartUTC,
		Notes:                   args.Notes,
		RescheduledFromID:       args.RescheduledFromID,
	}
	res, err := p.svc.CreateReservation(ctx, cmd)
	if err != nil {
		if err == ErrSlotTaken {
			return errResult("The selected time slot is already taken"), nil
		}
		return errResult(err.Error()), nil
	}
	var buf fmt.Builder
	if err := json.Encode(&res, &buf); err != nil {
		return errResult(err.Error()), nil
	}
	return mcp.Text(buf.String()), nil
}

func (p *ReservationProvider) getReservation(ctx *context.Context, req mcp.Request) (*mcp.Result, error) {
	var args getReservationArgs
	if err := req.Bind(&args); err != nil {
		return errResult("invalid arguments"), nil
	}
	res, err := p.svc.GetReservation(ctx, args.TenantID, args.ID)
	if err != nil {
		return errResult(err.Error()), nil
	}
	var buf fmt.Builder
	if err := json.Encode(&res, &buf); err != nil {
		return errResult(err.Error()), nil
	}
	return mcp.Text(buf.String()), nil
}

func (p *ReservationProvider) listReservationsByStaff(ctx *context.Context, req mcp.Request) (*mcp.Result, error) {
	var args listReservationsByStaffArgs
	if err := req.Bind(&args); err != nil {
		return errResult("invalid arguments"), nil
	}
	res, err := p.svc.ListReservationsByStaff(ctx, args.TenantID, args.StaffID, args.From, args.To)
	if err != nil {
		return errResult(err.Error()), nil
	}
	var list ReservationList
	for _, r := range res {
		r2 := r
		list = append(list, &r2)
	}
	var buf fmt.Builder
	if err := json.Encode(&list, &buf); err != nil {
		return errResult(err.Error()), nil
	}
	return mcp.Text(buf.String()), nil
}

func (p *ReservationProvider) listReservationsByClient(ctx *context.Context, req mcp.Request) (*mcp.Result, error) {
	var args listReservationsByClientArgs
	if err := req.Bind(&args); err != nil {
		return errResult("invalid arguments"), nil
	}
	res, err := p.svc.ListReservationsByClient(ctx, args.TenantID, args.ClientID)
	if err != nil {
		return errResult(err.Error()), nil
	}
	var list ReservationList
	for _, r := range res {
		r2 := r
		list = append(list, &r2)
	}
	var buf fmt.Builder
	if err := json.Encode(&list, &buf); err != nil {
		return errResult(err.Error()), nil
	}
	return mcp.Text(buf.String()), nil
}

func (p *ReservationProvider) changeReservationStatus(ctx *context.Context, req mcp.Request) (*mcp.Result, error) {
	var args changeReservationStatusArgs
	if err := req.Bind(&args); err != nil {
		return errResult("invalid arguments"), nil
	}
	cmd := ChangeStatusCmd{
		TenantID:  args.TenantID,
		ID:        args.ID,
		Event:     args.Event,
		ActorID:   args.ActorID,
		PaymentID: args.PaymentID,
		Revision:  int(args.Revision),
	}
	if err := p.svc.ChangeReservationStatus(ctx, cmd); err != nil {
		return errResult(err.Error()), nil
	}
	return mcp.Text("Status updated successfully"), nil
}

func (p *ReservationProvider) expirePendingReservations(ctx *context.Context, req mcp.Request) (*mcp.Result, error) {
	var args expirePendingReservationsArgs
	if err := req.Bind(&args); err != nil {
		return errResult("invalid arguments"), nil
	}
	count, err := p.svc.ExpirePendingReservations(ctx, args.TenantID, args.Before)
	if err != nil {
		return errResult(err.Error()), nil
	}
	return mcp.Text(fmt.Sprintf("Expired %d reservations", count)), nil
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
	var args upsertCalendarConfigArgs
	if err := req.Bind(&args); err != nil {
		return errResult("invalid arguments"), nil
	}
	cfg := WorkCalendarConfig{
		TenantID: args.TenantID,
		StaffID:  args.StaffID,
		Timezone: args.Timezone,
		IsActive: args.IsActive,
	}
	if err := p.svc.UpsertCalendarConfig(ctx, cfg); err != nil {
		return errResult(err.Error()), nil
	}
	return mcp.Text("Calendar config upserted successfully"), nil
}

func (p *CalendarProvider) upsertWeeklyCalendar(ctx *context.Context, req mcp.Request) (*mcp.Result, error) {
	var args upsertWeeklyCalendarArgs
	if err := req.Bind(&args); err != nil {
		return errResult("invalid arguments"), nil
	}
	cal := WorkCalendarWeekly{
		TenantID:    args.TenantID,
		StaffID:     args.StaffID,
		DayOfWeek:   args.DayOfWeek,
		WorkStart:   args.WorkStart,
		WorkFinish:  args.WorkFinish,
		BreakStart:  args.BreakStart,
		BreakFinish: args.BreakFinish,
		IsActive:    args.IsActive,
	}
	if err := p.svc.UpsertWeeklyCalendar(ctx, cal); err != nil {
		if err == ErrCalendarConfigNotFound {
			return errResult("Set the staff timezone first using upsert_calendar_config"), nil
		}
		return errResult(err.Error()), nil
	}
	return mcp.Text("Weekly calendar upserted successfully"), nil
}

func (p *CalendarProvider) addCalendarException(ctx *context.Context, req mcp.Request) (*mcp.Result, error) {
	var args addCalendarExceptionArgs
	if err := req.Bind(&args); err != nil {
		return errResult("invalid arguments"), nil
	}
	exc := WorkCalendarException{
		TenantID:      args.TenantID,
		StaffID:       args.StaffID,
		SpecificDate:  args.SpecificDate,
		ExceptionType: args.ExceptionType,
		StartTime:     args.StartTime,
		EndTime:       args.EndTime,
		Notes:         args.Notes,
	}
	if err := p.svc.AddException(ctx, exc); err != nil {
		return errResult(err.Error()), nil
	}
	return mcp.Text("Calendar exception added successfully"), nil
}

func (p *CalendarProvider) removeCalendarException(ctx *context.Context, req mcp.Request) (*mcp.Result, error) {
	var args removeCalendarExceptionArgs
	if err := req.Bind(&args); err != nil {
		return errResult("invalid arguments"), nil
	}
	if err := p.svc.RemoveException(ctx, args.TenantID, args.ExceptionID); err != nil {
		return errResult(err.Error()), nil
	}
	return mcp.Text("Calendar exception removed successfully"), nil
}

func (p *CalendarProvider) listAvailability(ctx *context.Context, req mcp.Request) (*mcp.Result, error) {
	var args listAvailabilityArgs
	if err := req.Bind(&args); err != nil {
		return errResult("invalid arguments"), nil
	}
	slots, err := p.svc.ListAvailability(ctx, args.TenantID, args.StaffID, args.ConfigID, args.From, args.To)
	if err != nil {
		if err == ErrCalendarConfigNotFound {
			return errResult("Set the staff timezone first using upsert_calendar_config"), nil
		}
		return errResult(err.Error()), nil
	}
	var list TimeSlotList
	for _, s := range slots {
		s2 := s
		list = append(list, &s2)
	}
	var buf fmt.Builder
	if err := json.Encode(&list, &buf); err != nil {
		return errResult(err.Error()), nil
	}
	return mcp.Text(buf.String()), nil
}
