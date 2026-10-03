declare global {
	namespace App {
		interface Platform {
			context?: Pick<ExecutionContext, 'waitUntil'>;
			env: {
				CLOUDFLARE_DOMAIN: string;
			};
		}
	}
}

export {};
