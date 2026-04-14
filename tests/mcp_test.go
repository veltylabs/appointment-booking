//go:build !wasm

package tests

import (
	"context"
	"testing"
	"encoding/json"

	"github.com/tinywasm/mcp"
	tinyctx "github.com/tinywasm/context"
	ab "github.com/veltylabs/appointment-booking"
)

type mockService struct {
	ab.SchedulingService
	errToReturn error
}

func (m *mockService) UpsertWeeklyCalendar(ctx context.Context, cal ab.WorkCalendarWeekly) error {
	if cal.StaffID == "" {
		return ab.ErrCalendarConfigNotFound
	}
	return m.errToReturn
}

func (m *mockService) CreateReservation(ctx context.Context, cmd ab.CreateReservationCmd) (ab.Reservation, error) {
	if cmd.SlotStartUTC == 1712000000 {
		return ab.Reservation{}, ab.ErrSlotTaken
	}
	return ab.Reservation{ID: "new-id"}, m.errToReturn
}

func newTestServer(t *testing.T, svc ab.SchedulingService) *mcp.Server {
	t.Helper()
	srv, err := mcp.NewServer(mcp.Config{
		Name:    "test",
		Version: "1.0.0",
		Auth:    mcp.OpenAuthorizer(),
	}, []mcp.ToolProvider{
		ab.NewCalendarProvider(svc),
		ab.NewReservationProvider(svc),
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return srv
}

func callTool(t *testing.T, srv *mcp.Server, name string, argsJSON string) *mcp.Result {
	t.Helper()
	ctx := tinyctx.Background()
    // Set dummy tokens to pass authorization
    ctx.Set("mcp.auth_token", "test-token")

	body := []byte(`{"jsonrpc":"2.0","id":"1","method":"tools/call","params":{"name":"` + name + `","arguments":` + json_quote(argsJSON) + `}}`)
	resp := srv.HandleMessage(ctx, body)

    b, _ := json.Marshal(resp)

    var envelope struct {
        Result string
        Error string
    }
    json.Unmarshal(b, &envelope)

    if envelope.Error != "" {
        var errDetails struct {
            Message string
        }
        json.Unmarshal([]byte(envelope.Error), &errDetails)
        return &mcp.Result{IsError: true, Content: errDetails.Message}
    }

    var res mcp.Result
    if err := json.Unmarshal([]byte(envelope.Result), &res); err != nil {
        return &mcp.Result{IsError: true, Content: "failed to decode result: " + err.Error()}
    }

	return &res
}

func json_quote(s string) string {
    b, _ := json.Marshal(s)
    return string(b)
}

func TestMCPHandlers(t *testing.T) {
	svc := &mockService{}
	srv := newTestServer(t, svc)

	t.Run("upsert_weekly_calendar_no_config", func(t *testing.T) {
		res := callTool(t, srv, "upsert_weekly_calendar", `{"tenant_id":"t1","staff_id":"","day_of_week":1,"work_start":540,"work_finish":1020,"break_start":0,"break_finish":0,"is_active":true}`)
		if res.Content != "Set the staff timezone first using upsert_calendar_config" {
			t.Fatalf("unexpected error message: %s", res.Content)
		}
	})

	t.Run("create_reservation_slot_taken", func(t *testing.T) {
		res := callTool(t, srv, "create_reservation", `{"tenant_id":"t1","client_id":"c1","creator_user_id":"u1","employee_service_config_id":"esc1","slot_start_utc":1712000000}`)
		if res.Content != "The selected time slot is already taken" {
			t.Fatalf("unexpected error message: %s", res.Content)
		}
	})
}
