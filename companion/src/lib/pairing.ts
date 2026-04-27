export type PairingPayload = {
	deviceURL: string;
	code: string;
};

export type CompanionStatus = {
	paired: boolean;
	deviceURL?: string;
	companionID?: string;
	localOnly?: boolean;
	capabilities?: Array<{ name: string }>;
	browserRuntimeStatus?: string;
	browserRuntimeError?: string;
};

export function parsePairingLink(value: string): PairingPayload {
	const parsedURL = new URL(value);
	if (parsedURL.protocol !== 'internkim:' || parsedURL.hostname !== 'pair') {
		throw new Error('Pairing link must start with internkim://pair');
	}
	const deviceURL = parsedURL.searchParams.get('device_url')?.trim() ?? '';
	const code = parsedURL.searchParams.get('code')?.trim() ?? '';
	if (!deviceURL || !code) {
		throw new Error('Pairing link requires device_url and code');
	}
	const parsedDeviceURL = new URL(deviceURL);
	if (parsedDeviceURL.protocol !== 'https:' && parsedDeviceURL.protocol !== 'http:') {
		throw new Error('Device URL must use http or https');
	}
	return { deviceURL, code };
}

export function normalizeManualPairingInput(deviceURL: string, code: string): PairingPayload {
	const normalizedDeviceURL = deviceURL.trim().replace(/\/+$/, '');
	const normalizedCode = code.trim().toUpperCase();
	if (!normalizedDeviceURL || !normalizedCode) {
		throw new Error('Device URL and pairing code are required');
	}
	const parsedDeviceURL = new URL(normalizedDeviceURL);
	if (parsedDeviceURL.protocol !== 'https:' && parsedDeviceURL.protocol !== 'http:') {
		throw new Error('Device URL must use http or https');
	}
	return { deviceURL: normalizedDeviceURL, code: normalizedCode };
}

export function statusLabel(status: CompanionStatus): string {
	if (!status.paired) return 'Not connected';
	if (!status.deviceURL) return 'Connected';
	return `Connected to ${status.deviceURL}`;
}
