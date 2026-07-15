//go:build !wasm

package tests

import (
	tinyctx "github.com/tinywasm/context"
	"testing"

	"github.com/tinywasm/mcp"
	ab "github.com/veltylabs/appointment_booking"
)

type mockService struct {
	ab.SchedulingService
	errToReturn error
}

func (m *mockService) UpsertWeeklyCalendar(ctx *tinyctx.Context, cal ab.WorkCalendarWeekly) error {
	if cal.StaffID == "" {
		return ab.ErrCalendarConfigNotFound
	}
	return m.errToReturn
}

func (m *mockService) CreateReservation(ctx *tinyctx.Context, cmd ab.CreateReservationCmd) (ab.Reservation, error) {
	if cmd.SlotStartUTC == 1712000000 {
		return ab.Reservation{}, ab.ErrSlotTaken
	}
	return ab.Reservation{ID: "new-id"}, m.errToReturn
}

func callTool(t *testing.T, providers []mcp.ToolProvider, name string, argsJSON string) *mcp.Result {
	t.Helper()
	for _, provider := range providers {
		for _, tool := range provider.Tools() {
			if tool.Name != name {
				continue
			}
			ctx := tinyctx.Background()
			req := mcp.Request{}
			req.Params.Arguments = argsJSON
			res, err := tool.Execute(ctx, req)
			if err != nil {
				t.Fatalf("Execute returned error: %v", err)
			}
			return res
		}
	}
	t.Fatalf("tool %q not found", name)
	return nil
}

func TestMCPHandlers(t *testing.T) {
	svc := &mockService{}
	providers := []mcp.ToolProvider{
		ab.NewCalendarProvider(svc),
		ab.NewReservationProvider(svc),
	}

	t.Run("upsert_weekly_calendar_no_config", func(t *testing.T) {
		res := callTool(t, providers, "upsert_weekly_calendar", `{"tenant_id":"t1","staff_id":"","day_of_week":1,"work_start":540,"work_finish":1020,"break_start":0,"break_finish":0,"is_active":true}`)
		if res.Content != "Set the staff timezone first using upsert_calendar_config" {
			t.Fatalf("unexpected error message: %s", res.Content)
		}
	})

	t.Run("create_reservation_slot_taken", func(t *testing.T) {
		res := callTool(t, providers, "create_reservation", `{"tenant_id":"t1","client_id":"c1","creator_user_id":"u1","employee_service_config_id":"esc1","slot_start_utc":1712000000}`)
		if res.Content != "The selected time slot is already taken" {
			t.Fatalf("unexpected error message: %s", res.Content)
		}
	})
}
