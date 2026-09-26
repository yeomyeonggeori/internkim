import { z } from 'zod';
import { supabase } from '$lib/supabase';
import { connectedBoxSchema, emptyBoxSchema, type ConnectedBox, type EmptyBox } from '$lib/company/box';
import { hostConfigurationSchema, hostSetupStatusSchema, type HostConfiguration } from '$lib/company/host-setup';
import { freshSealingMaterial, sealModelKey } from '$lib/company/seal-model-key';

async function askCompanyRoute(path: string, method: 'GET' | 'POST' | 'PUT', body?: unknown): Promise<unknown> {
	const { data } = await supabase().auth.getSession();
	if (!data.session) throw new Error('sign in first');
	const response = await fetch(path, {
		method,
		headers: { Authorization: `Bearer ${data.session.access_token}`, 'Content-Type': 'application/json' },
		...(body === undefined ? {} : { body: JSON.stringify(body) })
	});
	const document: unknown = await response.json();
	if (response.ok) return document;
	const message = typeof document === 'object' && document !== null && 'message' in document
		&& typeof document.message === 'string' ? document.message : `${path} returned ${response.status}`;
	throw new Error(message);
}

export async function fetchHostSetupStatus() {
	return hostSetupStatusSchema.parse(await askCompanyRoute('/api/company/host-setup', 'GET'));
}

export async function requestHostConfiguration(replaceExisting: boolean) {
	return hostConfigurationSchema.parse(await askCompanyRoute('/api/company/host-setup', 'POST', { replaceExisting }));
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

const boxesSchema = z.object({
	connected: connectedBoxSchema.nullable(),
	empty: z.array(emptyBoxSchema)
}).strict();

const connectedAnswerSchema = z.object({ connected: connectedBoxSchema.nullable() }).strict();

export type Boxes = { connected: ConnectedBox | null; empty: EmptyBox[] };

export async function fetchBoxes(): Promise<Boxes> {
	return boxesSchema.parse(await askCompanyRoute('/api/company/box', 'GET'));
}

export async function connectBox(publicKey: string): Promise<ConnectedBox | null> {
	return connectedAnswerSchema.parse(await askCompanyRoute('/api/company/box', 'POST', { publicKey })).connected;
}

export async function giveBoxModelKey(box: ConnectedBox, modelKey: string): Promise<ConnectedBox | null> {
	const sealed = await sealModelKey(modelKey.trim(), box.encryptionKey, freshSealingMaterial());
	return connectedAnswerSchema.parse(await askCompanyRoute('/api/company/box/model-key', 'PUT', sealed)).connected;
}
