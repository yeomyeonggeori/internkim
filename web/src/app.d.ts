declare global {
	namespace App {
		interface PageState {
			openThreadRootID?: string;
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
