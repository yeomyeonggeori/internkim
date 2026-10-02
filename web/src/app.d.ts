declare global {
	namespace App {
		interface Platform {
			context?: Pick<ExecutionContext, 'waitUntil'>;
			env: {
				KV: KVNamespace;
				CLOUDFLARE_DOMAIN: string;
				INTERNKIM_REGISTER_SECRET: string;
			};
		}
	}
}

export {};
