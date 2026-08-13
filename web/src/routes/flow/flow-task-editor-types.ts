export type FlowTaskEditorOption = {
	value: string;
	label: string;
};

export type FlowTaskRelationshipsText = {
	title: string;
	parent: string;
	children: string;
	add: string;
	searchParent: string;
	searchChildren: string;
	noCandidates: string;
	createChild: string;
	connectSelected: string;
	removeRelationship: string;
	moreActions: string;
	closeSelector: string;
	updateError: string;
	progressLabel: string;
	discardChanges: string;
};

export type FlowTaskEditorText = {
	editTitle: string;
	detailTitle: string;
	createTitle: string;
	requestTitle: string;
	content: string;
	contentPlaceholder: string;
	goal: string;
	goalPlaceholder: string;
	requester: string;
	requesterUnavailable: string;
	status: string;
	business: string;
	businessFallback: string;
	type: string;
	size: string;
	startDate: string;
	endDate: string;
	participants: string;
	participantsPlaceholder: string;
	removeParticipantAction: string;
	relationships: FlowTaskRelationshipsText;
	requestReason: string;
	reason: string;
	dateRule: string;
	readOnly: string;
	deleteAction: string;
	deleteTitle: string;
	deleteDescription: string;
	deleteConfirm: string;
	cancel: string;
	deleteError: string;
	saving: string;
	deleting: string;
	save: string;
};
