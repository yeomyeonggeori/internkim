import { createClient, type SupabaseClient } from '@supabase/supabase-js';

type CentralPlane = { projectURL: string; publishableKey: string; vapidPublicKey: string; gatewayURL: string };

let resolved: CentralPlane | undefined;
let client: SupabaseClient | undefined;

function centralPlane(): CentralPlane {
	if (resolved) return resolved;
	resolved = { projectURL: '', publishableKey: '', vapidPublicKey: '', gatewayURL: '' };
	if (typeof document === 'undefined') return resolved;
	const carried = document.getElementById('central-plane')?.textContent;
	if (!carried) return resolved;
	try {
		const parsed = JSON.parse(carried) as Partial<CentralPlane>;
		resolved = {
			projectURL: typeof parsed.projectURL === 'string' ? parsed.projectURL : '',
			publishableKey: typeof parsed.publishableKey === 'string' ? parsed.publishableKey : '',
			vapidPublicKey: typeof parsed.vapidPublicKey === 'string' ? parsed.vapidPublicKey : '',
			gatewayURL: typeof parsed.gatewayURL === 'string' ? parsed.gatewayURL : ''
		};
	} catch {
		resolved = { projectURL: '', publishableKey: '', vapidPublicKey: '', gatewayURL: '' };
	}
	return resolved;
}

export function vapidPublicKey(): string {
	return centralPlane().vapidPublicKey;
}

export function gatewayURL(): string {
	return centralPlane().gatewayURL;
}

export function isSupabaseConfigured(): boolean {
	const plane = centralPlane();
	return Boolean(plane.projectURL && plane.publishableKey);
}

export function supabase(): SupabaseClient {
	const plane = centralPlane();
	if (!plane.projectURL || !plane.publishableKey) throw new Error('this page is not served by the central plane');
	client ??= createClient(plane.projectURL, plane.publishableKey, {
		auth: { experimental: { passkey: true } }
	});
	return client;
}
