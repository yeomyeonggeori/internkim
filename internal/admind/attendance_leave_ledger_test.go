package admind

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestAttendanceLeaveLedgerGrantReserveUseAndIdempotency(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	ctx := context.Background()
	employee := attendanceLeaveEmployee{Email: "STAFF@example.com", UserID: "user-1"}
	balance, errorValue := service.grantAttendanceLeave(ctx, attendanceLeaveGrant{
		Operation: attendanceLeaveOperation{
			OperationKey: "grant-2026",
			Employee:     employee,
			LeaveTypeID:  "annual",
			Amount:       15000,
			EffectiveOn:  "2026-01-01",
		},
		ExpiresOn: "2027-01-01",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if balance.GrantedMilliDays != 15000 || balance.AvailableMilliDays != 15000 {
		t.Fatalf("grant balance = %+v", balance)
	}
	reservation := attendanceLeaveOperation{
		OperationKey: "reserve-request-1",
		Employee:     employee,
		LeaveTypeID:  "annual",
		ReferenceID:  "request-1",
		Amount:       500,
		EffectiveOn:  "2026-03-02",
	}
	balance, errorValue = service.reserveAttendanceLeave(ctx, reservation)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if balance.AvailableMilliDays != 14500 || balance.ReservedMilliDays != 500 {
		t.Fatalf("reserved balance = %+v", balance)
	}
	if _, errorValue := service.reserveAttendanceLeave(ctx, reservation); errorValue != nil {
		t.Fatalf("idempotent reserve: %v", errorValue)
	}
	balance, errorValue = service.useAttendanceLeaveReservation(ctx, attendanceLeaveOperation{
		OperationKey: "use-request-1",
		Employee:     employee,
		LeaveTypeID:  "annual",
		ReferenceID:  "request-1",
		EffectiveOn:  "2026-03-02",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if balance.AvailableMilliDays != 14500 || balance.ReservedMilliDays != 0 || balance.UsedMilliDays != 500 {
		t.Fatalf("used balance = %+v", balance)
	}
	if _, errorValue := service.useAttendanceLeaveReservation(ctx, attendanceLeaveOperation{
		OperationKey: "use-request-1",
		Employee:     employee,
		LeaveTypeID:  "annual",
		ReferenceID:  "request-1",
		EffectiveOn:  "2026-03-02",
	}); errorValue != nil {
		t.Fatalf("idempotent use: %v", errorValue)
	}
	if _, errorValue := service.releaseAttendanceLeaveReservation(ctx, attendanceLeaveOperation{
		OperationKey: "release-request-1",
		Employee:     employee,
		LeaveTypeID:  "annual",
		ReferenceID:  "request-1",
		EffectiveOn:  "2026-03-02",
	}); !errors.Is(errorValue, errAttendanceLeaveReservationClosed) {
		t.Fatalf("release after use error = %v", errorValue)
	}
	entries, errorValue := service.readAttendanceLeaveLedger(ctx, employee, "annual")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(entries) != 3 || entries[0].AvailableAfterMilliDays != 15000 || entries[1].AvailableAfterMilliDays != 14500 || entries[2].AvailableAfterMilliDays != 14500 {
		t.Fatalf("ledger entries = %+v", entries)
	}
}

func TestAttendanceLeaveLedgerTransactionReservationRollsBackWithRequest(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	ctx := context.Background()
	employee := attendanceLeaveEmployee{Email: "staff@example.com", UserID: "user-1"}
	if _, errorValue := service.grantAttendanceLeave(ctx, attendanceLeaveGrant{
		Operation: attendanceLeaveOperation{
			OperationKey: "grant-request-transaction",
			Employee:     employee,
			LeaveTypeID:  "annual",
			Amount:       1000,
			EffectiveOn:  "2027-01-01",
		},
		ExpiresOn: "2028-01-01",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := service.openAttendanceLeaveMutationDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := reserveAttendanceLeaveInTransaction(ctx, transaction, attendanceLeaveOperation{
		OperationKey: "reserve-request-transaction",
		Employee:     employee,
		LeaveTypeID:  "annual",
		ReferenceID:  "request-transaction",
		Amount:       500,
		EffectiveOn:  "2027-05-03",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := transaction.Rollback(); errorValue != nil {
		t.Fatal(errorValue)
	}
	balance, errorValue := service.readAttendanceLeaveBalance(ctx, employee, "annual")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if balance.AvailableMilliDays != 1000 || balance.ReservedMilliDays != 0 {
		t.Fatalf("balance after rollback = %+v", balance)
	}
}

func TestAttendanceLeaveLedgerRestoresApprovedUseWithReleaseOperation(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	ctx := context.Background()
	employee := attendanceLeaveEmployee{Email: "staff@example.com", UserID: "user-1"}
	if _, errorValue := service.grantAttendanceLeave(ctx, attendanceLeaveGrant{
		Operation: attendanceLeaveOperation{
			OperationKey: "grant-approved-cancel",
			Employee:     employee,
			LeaveTypeID:  "annual",
			Amount:       1000,
			EffectiveOn:  "2027-01-01",
		},
		ExpiresOn: "2028-01-01",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.reserveAttendanceLeave(ctx, attendanceLeaveOperation{
		OperationKey: "reserve-approved-cancel",
		Employee:     employee,
		LeaveTypeID:  "annual",
		ReferenceID:  "request-approved-cancel",
		Amount:       500,
		EffectiveOn:  "2027-05-03",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.useAttendanceLeaveReservation(ctx, attendanceLeaveOperation{
		OperationKey: "use-approved-cancel",
		Employee:     employee,
		LeaveTypeID:  "annual",
		ReferenceID:  "request-approved-cancel",
		EffectiveOn:  "2027-05-03",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := service.openAttendanceLeaveMutationDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	balance, errorValue := restoreAttendanceLeaveUseInTransaction(ctx, transaction, attendanceLeaveOperation{
		OperationKey: "restore-approved-cancel",
		Employee:     employee,
		LeaveTypeID:  "annual",
		ReferenceID:  "request-approved-cancel",
		EffectiveOn:  "2027-05-03",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		t.Fatal(errorValue)
	}
	if balance.AvailableMilliDays != 1000 || balance.UsedMilliDays != 0 {
		t.Fatalf("restored balance = %+v", balance)
	}
	entries, errorValue := service.readAttendanceLeaveLedger(ctx, employee, "annual")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if entries[len(entries)-1].Kind != attendanceLeaveOperationRelease ||
		entries[len(entries)-1].AvailableDeltaMilliDays != 500 ||
		entries[len(entries)-1].UsedDeltaMilliDays != -500 {
		t.Fatalf("restore entry = %+v", entries[len(entries)-1])
	}
}

func TestAttendanceLeaveLedgerReleaseExpiryAdjustmentAndUntrackedUse(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	ctx := context.Background()
	employee := attendanceLeaveEmployee{Email: "staff@example.com", UserID: "user-1"}
	if _, errorValue := service.grantAttendanceLeave(ctx, attendanceLeaveGrant{
		Operation: attendanceLeaveOperation{
			OperationKey: "grant-expiring",
			Employee:     employee,
			LeaveTypeID:  "annual",
			Amount:       2000,
			EffectiveOn:  "2026-01-01",
		},
		ExpiresOn: "2026-12-31",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.reserveAttendanceLeave(ctx, attendanceLeaveOperation{
		OperationKey: "reserve-release",
		Employee:     employee,
		LeaveTypeID:  "annual",
		ReferenceID:  "request-release",
		Amount:       250,
		EffectiveOn:  "2026-06-01",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	balance, errorValue := service.releaseAttendanceLeaveReservation(ctx, attendanceLeaveOperation{
		OperationKey: "release-reservation",
		Employee:     employee,
		LeaveTypeID:  "annual",
		ReferenceID:  "request-release",
		EffectiveOn:  "2026-06-01",
	})
	if errorValue != nil || balance.AvailableMilliDays != 2000 || balance.ReservedMilliDays != 0 {
		t.Fatalf("released balance = %+v, error = %v", balance, errorValue)
	}
	balance, errorValue = service.adjustAttendanceLeave(ctx, attendanceLeaveOperation{
		OperationKey: "adjust-minus",
		Employee:     employee,
		LeaveTypeID:  "annual",
		EffectiveOn:  "2026-08-01",
	}, -500, "")
	if errorValue != nil || balance.AvailableMilliDays != 1500 {
		t.Fatalf("adjusted balance = %+v, error = %v", balance, errorValue)
	}
	balance, errorValue = service.expireAttendanceLeave(ctx, attendanceLeaveOperation{
		OperationKey: "expire-year",
		Employee:     employee,
		LeaveTypeID:  "annual",
		EffectiveOn:  "2026-12-31",
	})
	if errorValue != nil || balance.AvailableMilliDays != 0 || balance.ExpiredMilliDays != 1500 {
		t.Fatalf("expired balance = %+v, error = %v", balance, errorValue)
	}
	balance, errorValue = service.expireAttendanceLeave(ctx, attendanceLeaveOperation{
		OperationKey: "expire-year",
		Employee:     employee,
		LeaveTypeID:  "annual",
		EffectiveOn:  "2026-12-31",
	})
	if errorValue != nil || balance.AvailableMilliDays != 0 || balance.ExpiredMilliDays != 1500 {
		t.Fatalf("idempotent expired balance = %+v, error = %v", balance, errorValue)
	}
	untracked, errorValue := service.recordUntrackedAttendanceLeaveUse(ctx, attendanceLeaveOperation{
		OperationKey: "sick-use",
		Employee:     employee,
		LeaveTypeID:  "sick",
		ReferenceID:  "sick-request",
		Amount:       1000,
		EffectiveOn:  "2026-09-01",
	})
	if errorValue != nil || untracked.AvailableMilliDays != 0 || untracked.UsedMilliDays != 1000 {
		t.Fatalf("untracked balance = %+v, error = %v", untracked, errorValue)
	}
	entries, errorValue := service.readAttendanceLeaveLedger(ctx, employee, "sick")
	if errorValue != nil || len(entries) != 1 || entries[0].UsedDeltaMilliDays != 1000 {
		t.Fatalf("untracked ledger = %+v, error = %v", entries, errorValue)
	}
}

func TestAttendanceLeaveLedgerCarryoverAppliesLimitAndExpiresRemainder(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	ctx := context.Background()
	employee := attendanceLeaveEmployee{Email: "staff@example.com", UserID: "user-1"}
	if _, errorValue := service.grantAttendanceLeave(ctx, attendanceLeaveGrant{
		Operation: attendanceLeaveOperation{
			OperationKey: "grant-carryover",
			Employee:     employee,
			LeaveTypeID:  "annual",
			Amount:       5000,
			EffectiveOn:  "2026-01-01",
		},
		ExpiresOn: "2027-01-01",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	limit := 3000
	operation := attendanceLeaveOperation{
		OperationKey: "carryover-2027",
		Employee:     employee,
		LeaveTypeID:  "annual",
		EffectiveOn:  "2027-01-01",
	}
	balance, errorValue := service.carryoverAttendanceLeave(ctx, operation, &limit, "2028-01-01")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if balance.GrantedMilliDays != 5000 || balance.AvailableMilliDays != 3000 || balance.ExpiredMilliDays != 2000 {
		t.Fatalf("carryover balance = %+v", balance)
	}
	if _, errorValue := service.reserveAttendanceLeave(ctx, attendanceLeaveOperation{
		OperationKey: "reserve-carryover",
		Employee:     employee,
		LeaveTypeID:  "annual",
		ReferenceID:  "request-carryover",
		Amount:       500,
		EffectiveOn:  "2027-02-01",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.useAttendanceLeaveReservation(ctx, attendanceLeaveOperation{
		OperationKey: "use-carryover",
		Employee:     employee,
		LeaveTypeID:  "annual",
		ReferenceID:  "request-carryover",
		EffectiveOn:  "2027-02-01",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.carryoverAttendanceLeave(ctx, operation, &limit, "2028-01-01"); errorValue != nil {
		t.Fatalf("idempotent carryover: %v", errorValue)
	}
	entries, errorValue := service.readAttendanceLeaveLedger(ctx, employee, "annual")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	kinds := []string{}
	for _, entry := range entries {
		kinds = append(kinds, entry.Kind)
	}
	expectedKinds := []string{"grant", "carryoverOut", "expire", "carryoverIn", "reserve", "use"}
	if len(kinds) != len(expectedKinds) {
		t.Fatalf("carryover ledger kinds = %v", kinds)
	}
	for index := range expectedKinds {
		if kinds[index] != expectedKinds[index] {
			t.Fatalf("carryover ledger kinds = %v", kinds)
		}
	}
}

func TestAttendanceLeaveLedgerRejectsInsufficientAndConcurrentReservations(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	ctx := context.Background()
	employee := attendanceLeaveEmployee{Email: "staff@example.com", UserID: "user-1"}
	if _, errorValue := service.grantAttendanceLeave(ctx, attendanceLeaveGrant{
		Operation: attendanceLeaveOperation{
			OperationKey: "grant-concurrent",
			Employee:     employee,
			LeaveTypeID:  "annual",
			Amount:       1000,
			EffectiveOn:  "2026-01-01",
		},
		ExpiresOn: "2027-01-01",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	type result struct {
		balance attendanceLeaveBalance
		error   error
	}
	results := make(chan result, 2)
	start := make(chan struct{})
	var waitGroup sync.WaitGroup
	for index := 1; index <= 2; index++ {
		waitGroup.Add(1)
		go func(requestIndex int) {
			defer waitGroup.Done()
			<-start
			balance, errorValue := service.reserveAttendanceLeave(ctx, attendanceLeaveOperation{
				OperationKey: "concurrent-" + string(rune('0'+requestIndex)),
				Employee:     employee,
				LeaveTypeID:  "annual",
				ReferenceID:  "concurrent-request-" + string(rune('0'+requestIndex)),
				Amount:       750,
				EffectiveOn:  "2026-04-01",
			})
			results <- result{balance: balance, error: errorValue}
		}(index)
	}
	close(start)
	waitGroup.Wait()
	close(results)
	successes := 0
	insufficient := 0
	for reservationResult := range results {
		switch {
		case reservationResult.error == nil:
			successes++
		case errors.Is(reservationResult.error, errAttendanceLeaveInsufficientBalance):
			insufficient++
		default:
			t.Fatalf("unexpected reservation error: %v", reservationResult.error)
		}
	}
	if successes != 1 || insufficient != 1 {
		t.Fatalf("successes = %d, insufficient = %d", successes, insufficient)
	}
	balance, errorValue := service.readAttendanceLeaveBalance(ctx, employee, "annual")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if balance.AvailableMilliDays != 250 || balance.ReservedMilliDays != 750 {
		t.Fatalf("concurrent balance = %+v", balance)
	}
}

func TestAttendanceLeaveLedgerAllowsOnlyOneConcurrentReservationTerminal(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	ctx := context.Background()
	employee := attendanceLeaveEmployee{Email: "staff@example.com", UserID: "user-1"}
	if _, errorValue := service.grantAttendanceLeave(ctx, attendanceLeaveGrant{
		Operation: attendanceLeaveOperation{
			OperationKey: "grant-terminal",
			Employee:     employee,
			LeaveTypeID:  "annual",
			Amount:       1000,
			EffectiveOn:  "2026-01-01",
		},
		ExpiresOn: "2027-01-01",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.reserveAttendanceLeave(ctx, attendanceLeaveOperation{
		OperationKey: "reserve-terminal",
		Employee:     employee,
		LeaveTypeID:  "annual",
		ReferenceID:  "terminal-request",
		Amount:       500,
		EffectiveOn:  "2026-04-01",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	results := make(chan error, 2)
	start := make(chan struct{})
	var waitGroup sync.WaitGroup
	waitGroup.Add(2)
	go func() {
		defer waitGroup.Done()
		<-start
		_, errorValue := service.useAttendanceLeaveReservation(ctx, attendanceLeaveOperation{
			OperationKey: "use-terminal",
			Employee:     employee,
			LeaveTypeID:  "annual",
			ReferenceID:  "terminal-request",
			EffectiveOn:  "2026-04-01",
		})
		results <- errorValue
	}()
	go func() {
		defer waitGroup.Done()
		<-start
		_, errorValue := service.releaseAttendanceLeaveReservation(ctx, attendanceLeaveOperation{
			OperationKey: "release-terminal",
			Employee:     employee,
			LeaveTypeID:  "annual",
			ReferenceID:  "terminal-request",
			EffectiveOn:  "2026-04-01",
		})
		results <- errorValue
	}()
	close(start)
	waitGroup.Wait()
	close(results)
	successes := 0
	closed := 0
	for errorValue := range results {
		switch {
		case errorValue == nil:
			successes++
		case errors.Is(errorValue, errAttendanceLeaveReservationClosed):
			closed++
		default:
			t.Fatalf("unexpected terminal error: %v", errorValue)
		}
	}
	if successes != 1 || closed != 1 {
		t.Fatalf("terminal successes = %d, closed = %d", successes, closed)
	}
	balance, errorValue := service.readAttendanceLeaveBalance(ctx, employee, "annual")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if balance.ReservedMilliDays != 0 {
		t.Fatalf("terminal balance = %+v", balance)
	}
	isUsed := balance.AvailableMilliDays == 500 && balance.UsedMilliDays == 500
	isReleased := balance.AvailableMilliDays == 1000 && balance.UsedMilliDays == 0
	if !isUsed && !isReleased {
		t.Fatalf("terminal balance = %+v", balance)
	}
}

func TestAttendanceLeaveLedgerSchemaAndOperationConflict(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	ctx := context.Background()
	employee := attendanceLeaveEmployee{Email: "staff@example.com", UserID: "user-1"}
	grant := attendanceLeaveGrant{
		Operation: attendanceLeaveOperation{
			OperationKey: "grant-conflict",
			Employee:     employee,
			LeaveTypeID:  "annual",
			Amount:       1000,
			EffectiveOn:  "2026-01-01",
		},
		ExpiresOn: "2027-01-01",
	}
	balance, errorValue := service.grantAttendanceLeave(ctx, grant)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	grant.Operation.Amount = 2000
	if _, errorValue := service.grantAttendanceLeave(ctx, grant); !errors.Is(errorValue, errAttendanceLeaveOperationConflict) {
		t.Fatalf("operation conflict error = %v", errorValue)
	}
	stored, errorValue := service.readAttendanceLeaveBalance(ctx, employee, "annual")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if stored != balance {
		t.Fatalf("balance changed after conflict: before=%+v after=%+v", balance, stored)
	}
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var foreignKeyErrors int
	if errorValue := database.QueryRowContext(ctx, "SELECT COUNT(*) FROM pragma_foreign_key_check").Scan(&foreignKeyErrors); errorValue != nil {
		t.Fatal(errorValue)
	}
	if foreignKeyErrors != 0 {
		t.Fatalf("foreign key errors = %d", foreignKeyErrors)
	}
	if _, errorValue := database.ExecContext(ctx, `
INSERT INTO attendance_leave_operations (
	operation_key, employee_email, user_id, leave_type_id, kind, reference_id,
	amount_milli_days, effective_date, created_at
) VALUES ('invalid-kind', 'staff@example.com', 'user-1', 'annual', 'invalid', '', 0, '2026-01-01', '2026-01-01T00:00:00Z')`); errorValue == nil {
		t.Fatal("expected operation kind constraint failure")
	}
}
