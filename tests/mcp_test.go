//go:build !wasm

package tests

import (
	ab "github.com/veltylabs/appointment-booking"
	"context"
	"testing"
    "fmt"

	"github.com/tinywasm/mcp"
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

func TestMCPHandlers(t *testing.T) {
	s := mcp.NewMCPServer("test", "1.0")
	svc := &mockService{}
	ab.Register(s, svc)

	client, err := mcp.NewInProcessClient(s)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()
	_, err = client.Initialize(ctx, mcp.InitializeRequest{})
	if err != nil {
		t.Fatalf("Failed to initialize client: %v", err)
	}

	t.Run("upsert_weekly_calendar_no_config", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "upsert_weekly_calendar",
				Arguments: map[string]any{
					"tenant_id":    "t1",
					"staff_id":     "", // empty -> triggers ab.ErrCalendarConfigNotFound
					"day_of_week":  1,
					"work_start":   540,
					"work_finish":  1020,
					"break_start":  0,
					"break_finish": 0,
					"is_active":    true,
				},
			},
		}

		res, err := client.CallTool(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error calling tool: %v", err)
		}
		if !res.IsError {
			t.Fatalf("expected error result")
		}

		txt := res.Content[0].(mcp.TextContent).Text
		if txt != "Set the staff timezone first using upsert_calendar_config" {
			t.Fatalf("unexpected error message: %s", txt)
		}
	})

	t.Run("create_reservation_slot_taken", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "create_reservation",
				Arguments: map[string]any{
					"tenant_id":                  "t1",
					"client_id":                  "c1",
					"creator_user_id":            "u1",
					"employee_service_config_id": "esc1",
					"slot_start_utc":             int64(1712000000), // triggers ab.ErrSlotTaken
				},
			},
		}

		res, err := client.CallTool(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error calling tool: %v", err)
		}
		if !res.IsError {
			t.Fatalf("expected error result")
		}

		txt := res.Content[0].(mcp.TextContent).Text
		if txt != "The selected time slot is already taken" {
			t.Fatalf("unexpected error message: %s", txt)
		}
	})
    fmt.Println("MCP tests done")
}
