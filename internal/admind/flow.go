package admind

const (
	flowActionRead   = "read"
	flowActionCreate = "create"
	flowActionUpdate = "update"
	flowActionDelete = "delete"
	flowActionManage = "manage"

	flowResourceSummary    = "api:flow.summary"
	flowResourceTask       = "api:flow.task"
	flowResourceDefinition = "api:flow.definition"

	flowRequesterEmailHeader = "X-InternKim-Requester-Email"
	flowResolvedActorHeader  = "X-InternKim-Resolved-Actor-Email"
)
