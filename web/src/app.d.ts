/// <reference lib="dom.asynciterable" />
declare global {
	namespace App {
		interface PageState {
			openThreadRootID?: string;
			openAttendanceTeamKey?: string;
		}
		interface Platform {
			context?: Pick<ExecutionContext, 'waitUntil'>;
			env: {
				CLOUDFLARE_DOMAIN: string;
			};
		}
	}
}

export {};
