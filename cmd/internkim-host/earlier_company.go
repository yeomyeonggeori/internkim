package main

import (
	"os"

	"github.com/yeomyeonggeori/internkim/internal/companyhost"
)

func forgetEveryCompany() error {
	return companyhost.ForgetEveryCompany(thisComputer{}, os.Stdout)
}
