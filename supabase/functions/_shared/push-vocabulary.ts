export type Notification = {
	title: string;
	body: string;
	openPath: string;
	tag: string;
	icon?: string;
};

export type PushOutcome = 'delivered' | 'gone' | 'refused';
