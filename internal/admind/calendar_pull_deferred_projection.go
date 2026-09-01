package admind

func (service *Service) runCalendarPullLocked(operation func()) {
	service.calendarStoreWriteMutex.Lock()
	defer service.calendarStoreWriteMutex.Unlock()
	operation()
}
