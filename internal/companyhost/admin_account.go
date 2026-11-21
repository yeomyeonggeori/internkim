package companyhost

import (
	"os/user"
	"slices"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

const AdminAccountName = "admin"

type AccountDirectory struct {
	LookupAccount func(name string) (*user.User, error)
	LookupGroup   func(name string) (*user.Group, error)
	GroupIDsOf    func(account *user.User) ([]string, error)
}

func (directory AccountDirectory) HasAdminAccount() bool {
	account, errorValue := directory.lookupAccount(AdminAccountName)
	if errorValue != nil {
		return false
	}
	group, errorValue := directory.lookupGroup(blueclaw.CompanyHostAdministratorGroup)
	if errorValue != nil {
		return false
	}
	groupIDs, errorValue := directory.groupIDsOf(account)
	if errorValue != nil {
		return false
	}
	return slices.Contains(groupIDs, group.Gid)
}

func (directory AccountDirectory) lookupAccount(name string) (*user.User, error) {
	if directory.LookupAccount != nil {
		return directory.LookupAccount(name)
	}
	return user.Lookup(name)
}

func (directory AccountDirectory) lookupGroup(name string) (*user.Group, error) {
	if directory.LookupGroup != nil {
		return directory.LookupGroup(name)
	}
	return user.LookupGroup(name)
}

func (directory AccountDirectory) groupIDsOf(account *user.User) ([]string, error) {
	if directory.GroupIDsOf != nil {
		return directory.GroupIDsOf(account)
	}
	return account.GroupIds()
}
