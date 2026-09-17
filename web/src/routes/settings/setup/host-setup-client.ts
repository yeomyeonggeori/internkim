import { supabase } from '$lib/supabase';
import { hostConfigurationSchema, hostSetupStatusSchema, type HostConfiguration } from '$lib/company/host-setup';

async function requestSetup(method: 'GET' | 'POST', replaceExisting = false): Promise<unknown> {
	const { data } = await supabase().auth.getSession();
	if (!data.session) throw new Error('sign in first');
	const response = await fetch('/api/company/host-setup', {
		method,
		headers: { Authorization: `Bearer ${data.session.access_token}`, 'Content-Type': 'application/json' },
		...(method === 'POST' ? { body: JSON.stringify({ replaceExisting }) } : {})
	});
	const document: unknown = await response.json();
	if (response.ok) return document;
	const message = typeof document === 'object' && document !== null && 'message' in document
		&& typeof document.message === 'string' ? document.message : `connection setup returned ${response.status}`;
	throw new Error(message);
}

export async function fetchHostSetupStatus() {
	return hostSetupStatusSchema.parse(await requestSetup('GET'));
}

export async function requestHostConfiguration(replaceExisting: boolean) {
	return hostConfigurationSchema.parse(await requestSetup('POST', replaceExisting));
}

export function downloadHostConfiguration(configuration: HostConfiguration): void {
	const address = URL.createObjectURL(new Blob([JSON.stringify(configuration, null, 2)], { type: 'application/json' }));
	const link = document.createElement('a');
	link.href = address;
	link.download = 'internkim-host.json';
	document.body.appendChild(link);
	link.click();
	link.remove();
	setTimeout(() => URL.revokeObjectURL(address), 1000);
}
