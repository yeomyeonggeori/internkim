package admind

func (service *Service) runCalendarStoreSideEffectUnlocked(operation func()) {
	service.calendarStoreWriteMutex.Unlock()
	defer service.calendarStoreWriteMutex.Lock()
	operation()
}
