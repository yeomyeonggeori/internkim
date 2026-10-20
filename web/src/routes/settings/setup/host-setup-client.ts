import { z } from 'zod';
import { supabase } from '$lib/supabase';
import {
	companyBoxesSchema,
	emptyBoxSchema,
	verifiedBoxSchema,
	type BoxClaimIntent,
	type BoxPlacement,
	type CompanyBoxes,
	type ConnectedBox,
	type EmptyBox,
	type TakeOver,
	type VerifiedBoxAnswer
} from '$lib/company/box';
import { hostConfigurationSchema, hostSetupStatusSchema, type HostConfiguration } from '$lib/company/host-setup';
import { sealModelKey } from '$lib/company/seal-to-box';

export async function askCompanyRoute(path: string, method: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE', body?: unknown): Promise<unknown> {
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

const boxesSchema = companyBoxesSchema.extend({ empty: z.array(emptyBoxSchema) }).strict();

export type Boxes = CompanyBoxes & { empty: EmptyBox[] };

export async function fetchBoxes(): Promise<Boxes> {
	return boxesSchema.parse(await askCompanyRoute('/api/company/box', 'GET'));
}

export async function verifyBoxCode(publicKey: string, pairingCode: string): Promise<VerifiedBoxAnswer> {
	return verifiedBoxSchema.parse(await askCompanyRoute('/api/company/box/verification', 'POST', { publicKey, pairingCode }));
}

export async function connectBox(publicKey: string, ticket: string, intent: BoxClaimIntent): Promise<CompanyBoxes> {
	return companyBoxesSchema.parse(await askCompanyRoute('/api/company/box', 'POST', { publicKey, ticket, intent }));
}

export async function disconnectBox(box: ConnectedBox): Promise<CompanyBoxes> {
	return companyBoxesSchema.parse(await askCompanyRoute('/api/company/box', 'DELETE', { publicKey: box.publicKey }));
}

export async function makeBoxServe(box: ConnectedBox): Promise<CompanyBoxes> {
	return companyBoxesSchema.parse(await askCompanyRoute('/api/company/box', 'PATCH', { publicKey: box.publicKey }));
}

export async function placeBox(box: ConnectedBox, placement: BoxPlacement): Promise<CompanyBoxes> {
	return companyBoxesSchema.parse(await askCompanyRoute('/api/company/box', 'PUT', { publicKey: box.publicKey, ...placement }));
}

export async function keepTakeOver(takeOver: TakeOver): Promise<CompanyBoxes> {
	return companyBoxesSchema.parse(await askCompanyRoute('/api/company/box/take-over', 'PUT', takeOver));
}

export async function giveModelKey(companyID: string, sealingKey: string, modelKey: string): Promise<CompanyBoxes> {
	const sealed = await sealModelKey(modelKey.trim(), { companyID, encryptionKey: sealingKey });
	return companyBoxesSchema.parse(await askCompanyRoute('/api/company/box/model-key', 'PUT', sealed));
}
