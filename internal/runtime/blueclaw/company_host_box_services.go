package blueclaw

const CompanyHostTakeOverServiceName = "internkim-take-over"

func CompanyHostBoxServices(layout CompanyHostLayout) []CompanyHostService {
	return []CompanyHostService{
		companyHostBoxService(layout),
		companyHostTakeOverService(layout),
	}
}

func CompanyHostDeclaredServiceNamed(layout CompanyHostLayout, name string) (CompanyHostService, bool) {
	for _, service := range append(CompanyHostServices(layout), CompanyHostBoxServices(layout)...) {
		if service.Name == name {
			return service, true
		}
	}
	return CompanyHostService{}, false
}

func companyHostBoxService(layout CompanyHostLayout) CompanyHostService {
	return CompanyHostService{
		Name:                BoxServiceName,
		Description:         "internkim box, which a company claims from its own network",
		Command:             []string{layout.BinaryPath(CompanyPackageName), "box"},
		RestartAfterSeconds: 30,
	}
}

func companyHostTakeOverService(layout CompanyHostLayout) CompanyHostService {
	return CompanyHostService{
		Name:                CompanyHostTakeOverServiceName,
		Description:         "internkim take-over, which restores the company onto this box",
		Command:             []string{layout.BinaryPath(CompanyPackageName), "restore", "--taking-over"},
		StartsOnlyWhenAsked: true,
	}
}
