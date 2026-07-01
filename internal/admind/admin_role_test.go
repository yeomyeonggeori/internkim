package admind

import "testing"

func TestNormalizeAdminUserRole(t *testing.T) {
	cases := []struct {
		name string
		role string
		want string
	}{
		{name: "admin", role: adminUserRoleAdmin, want: adminUserRoleAdmin},
		{name: "operations admin", role: adminUserRoleOperationsAdmin, want: adminUserRoleOperationsAdmin},
		{name: "member", role: adminUserRoleMember, want: adminUserRoleMember},
		{name: "unknown role", role: "owner", want: adminUserRoleMember},
		{name: "empty role", role: "", want: adminUserRoleMember},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := normalizeAdminUserRole(testCase.role)
			if got != testCase.want {
				t.Fatalf("role = %q, want %q", got, testCase.want)
			}
		})
	}
}
