declare global {
	namespace App {
		interface Platform {
			env: {
				KV: KVNamespace;
				CF_API_TOKEN: string;
				CF_ACCOUNT_ID: string;
				CF_ZONE_ID: string;
				CF_DOMAIN: string;
				INTERNKIM_REGISTER_SECRET: string;
			};
		}
	}
}

declare module 'svelte-qrcode' {
	const QrCode: unknown;
	export default QrCode;
}

export {};
