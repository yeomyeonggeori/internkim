package admind

func (service *Service) centralPlaneAgentKey() string {
	return readTrimmedFile(service.Configuration.CentralPlaneAgentKeyPath)
}
