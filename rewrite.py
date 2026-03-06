import os, re

for fname in os.listdir("tests"):
    if not fname.endswith("_test.go"): continue
    p = os.path.join("tests", fname)
    with open(p, "r") as f:
        content = f.read()
    
    # Replace package name
    content = content.replace("package appointmentbooking", "package tests")
    
    # Add import
    if 'import (' in content:
        content = content.replace('import (', 'import (\n\tab "github.com/veltylabs/appointment-booking"', 1)
    else:
        content = content.replace('import "testing"', 'import (\n\t"testing"\n\tab "github.com/veltylabs/appointment-booking"\n)')
    
    # Add ab. prefix to exported module types and functions
    # List of known exported symbols:
    symbols = [
        "StatusPending", "StatusConfirmed", "StatusCancelled", "StatusExpired", "StatusRescheduled", "StatusCompleted", "StatusNoShow",
        "EventConfirm", "EventCancel", "EventExpire", "EventReschedule", "EventComplete", "EventNoShow",
        "Transition", "ErrInvalidTransition", "IsTerminal",
        "Repository", "NewRepository",
        "EmployeeServiceConfig", "WorkCalendarConfig", "WorkCalendarWeekly", "WorkCalendarException", "Reservation", "TimeSlot",
        "SchedulingService", "Deps", "CreateReservationCmd", "ChangeStatusCmd",
        "ErrSlotTaken", "ErrCalendarConfigNotFound", "EventReservationCreated", "EventReservationConfirmed", "EventReservationCancelled", "EventReservationCompleted", "EventReservationNoShow", "EventReservationExpired", "EventReservationRescheduled",
        "New", "ErrNotFound", "ErrConflict"
    ]
    
    for sym in symbols:
        # Match word boundaries so we don't replace inside other words
        content = re.sub(r'\b' + sym + r'\b', 'ab.' + sym, content)
    
    with open(p, "w") as f:
        f.write(content)
