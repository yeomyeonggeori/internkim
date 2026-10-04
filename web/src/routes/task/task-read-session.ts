export type TaskReadContext = { scope: string; week: string; generation: number };
export type TaskReadTicket = TaskReadContext & { loadID: number };

export function sameTaskReadContext(left: TaskReadContext, right: TaskReadContext): boolean {
	return left.scope === right.scope && left.week === right.week && left.generation === right.generation;
}

export function createTaskReadSession() {
	let scope = '';
	let week = '';
	let loadID = 0;
	let fullHistoryRequested = false;
	return {
		select(nextScope: string, nextWeek: string): void {
			if (scope !== nextScope) fullHistoryRequested = false;
			if (scope !== nextScope || week !== nextWeek) loadID++;
			scope = nextScope;
			week = nextWeek;
		},
		requireFullHistory(): void { fullHistoryRequested = true; },
		needsFullHistory(): boolean { return fullHistoryRequested; },
		invalidate(): void { loadID++; },
		start(generation: number): TaskReadTicket { return { loadID: ++loadID, scope, week, generation }; },
		owns(ticket: TaskReadTicket): boolean { return ticket.loadID === loadID && ticket.scope === scope && ticket.week === week; },
		isCurrent(ticket: TaskReadTicket, generation: number): boolean {
			return this.owns(ticket) && ticket.generation === generation;
		}
	};
}
