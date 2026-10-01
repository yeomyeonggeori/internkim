declare global {
	namespace App {
		interface Platform {
			context?: Pick<ExecutionContext, 'waitUntil'>;
			env: {
				CLOUDFLARE_API_TOKEN: string;
				CLOUDFLARE_ACCOUNT_ID: string;
				CLOUDFLARE_ZONE_ID: string;
				CLOUDFLARE_DOMAIN: string;
			};
		}
	}
}

export {};
