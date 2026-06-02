export class CalendarProgrammaticUpdateState {
	private readonly eventUpdateCounts = new Map<string, number>();

	isActive(eventID: string): boolean {
		return (this.eventUpdateCounts.get(eventID) ?? 0) > 0;
	}

	async run<T>(eventID: string, update: () => Promise<T>): Promise<T> {
		this.begin(eventID);
		try {
			return await update();
		} finally {
			this.finish(eventID);
		}
	}

	private begin(eventID: string): void {
		this.eventUpdateCounts.set(eventID, (this.eventUpdateCounts.get(eventID) ?? 0) + 1);
	}

	private finish(eventID: string): void {
		const currentCount = this.eventUpdateCounts.get(eventID) ?? 0;
		if (currentCount <= 1) {
			this.eventUpdateCounts.delete(eventID);
			return;
		}
		this.eventUpdateCounts.set(eventID, currentCount - 1);
	}
}
