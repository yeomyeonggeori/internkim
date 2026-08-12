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

	flowRequesterEmailHeader = "X-INTERNKIM-REQUESTER-EMAIL"
	flowResolvedActorHeader  = "X-INTERNKIM-RESOLVED-ACTOR-EMAIL"
)
