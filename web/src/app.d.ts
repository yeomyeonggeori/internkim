declare global {
	namespace App {
		interface Platform {
			env: {
				KV: KVNamespace;
				CLOUDFLARE_API_TOKEN: string;
				CLOUDFLARE_ACCOUNT_ID: string;
				CLOUDFLARE_ZONE_ID: string;
				CLOUDFLARE_DOMAIN: string;
				INTERNKIM_REGISTER_SECRET: string;
			};
		}
	}
}

export {};
