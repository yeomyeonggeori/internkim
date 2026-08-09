package admind

import (
	"net/http"
	"strconv"
	"time"
)

type attendanceBackfillResponse struct {
	Months   []string `json:"months"`
	Failures []string `json:"failures"`
}

func (service *Service) backfillAttendanceCentrally(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.NotFound(responseWriter, request)
		return
	}
	if service.centralPlane() == nil {
		http.Error(responseWriter, "this device is not attached to a central plane", http.StatusPreconditionFailed)
		return
	}

	months := monthsToReconcile(time.Now().UTC(), backfillMonths(request.URL.Query().Get("months")))
	answer := attendanceBackfillResponse{Months: months, Failures: []string{}}
	for _, month := range months {
		if errorValue := service.reconcileAttendanceMonth(request.Context(), month); errorValue != nil {
			answer.Failures = append(answer.Failures, month+": "+errorValue.Error())
		}
	}
	service.writeJSON(responseWriter, answer)
}

func backfillMonths(asked string) int {
	const mostMonths = 60
	months, errorValue := strconv.Atoi(asked)
	if errorValue != nil || months <= 0 {
		return 11
	}
	if months > mostMonths {
		return mostMonths
	}
	return months
}
