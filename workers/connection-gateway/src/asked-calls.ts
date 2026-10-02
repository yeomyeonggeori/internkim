export type AskedCallStorage = Pick<SqlStorage, 'exec'>;

export const askedCallsKept = 4096;

export class AskedCalls {
	constructor(
		private readonly sql: AskedCallStorage,
		private readonly capacity = askedCallsKept
	) {
		sql.exec('CREATE TABLE IF NOT EXISTS asked_call (request_id TEXT PRIMARY KEY, member_id TEXT NOT NULL)');
	}

	remember(requestID: string, memberID: string): void {
		this.sql.exec('INSERT OR REPLACE INTO asked_call (request_id, member_id) VALUES (?, ?)', requestID, memberID);
		this.sql.exec(
			'DELETE FROM asked_call WHERE rowid <= (SELECT MAX(rowid) FROM asked_call) - ?',
			this.capacity
		);
	}

	take(requestID: string): string | undefined {
		const [row] = this.sql
			.exec<{ member_id: string }>('SELECT member_id FROM asked_call WHERE request_id = ?', requestID)
			.toArray();
		if (!row) return undefined;
		this.forget(requestID);
		return row.member_id;
	}

	forget(requestID: string): void {
		this.sql.exec('DELETE FROM asked_call WHERE request_id = ?', requestID);
	}

	requestIDs(): string[] {
		return this.sql
			.exec<{ request_id: string }>('SELECT request_id FROM asked_call')
			.toArray()
			.map((row) => row.request_id);
	}
}
