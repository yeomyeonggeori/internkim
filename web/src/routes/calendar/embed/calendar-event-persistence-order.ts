export class CalendarEventPersistenceOrder {
	private readonly latestActionRevisions = new Map<string, number>();
	private readonly pendingActions = new Map<string, Promise<void>>();
	private readonly persistedUpdatedAtByEventID = new Map<string, string>();

	beginAction(eventID: string): number {
		const revision = (this.latestActionRevisions.get(eventID) ?? 0) + 1;
		this.latestActionRevisions.set(eventID, revision);
		return revision;
	}

	isLatestAction(eventID: string, revision: number): boolean {
		return this.latestActionRevisions.get(eventID) === revision;
	}

	recordPersistedUpdatedAt(eventID: string, updatedAt: string): void {
		this.persistedUpdatedAtByEventID.set(eventID, updatedAt);
	}

	persistedUpdatedAt(eventID: string, fallback: string | undefined): string | undefined {
		return this.persistedUpdatedAtByEventID.get(eventID) ?? fallback;
	}

	clearPersistedUpdatedAt(eventID: string, updatedAt?: string): void {
		if (updatedAt !== undefined && this.persistedUpdatedAtByEventID.get(eventID) !== updatedAt) return;
		this.persistedUpdatedAtByEventID.delete(eventID);
	}

	async runLatestAction(eventID: string, revision: number, action: () => Promise<void>): Promise<void> {
		const previousAction = this.pendingActions.get(eventID) ?? Promise.resolve();
		const pendingAction = previousAction
			.catch(() => {})
			.then(async () => {
				if (!this.isLatestAction(eventID, revision)) return;
				await action();
			});
		this.pendingActions.set(eventID, pendingAction);
		try {
			await pendingAction;
		} finally {
			if (this.pendingActions.get(eventID) === pendingAction) {
				this.pendingActions.delete(eventID);
			}
		}
	}
}
