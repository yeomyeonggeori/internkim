const CF_API = 'https://api.cloudflare.com/client/v4';

interface CFEnv {
	CF_API_TOKEN: string;
	CF_ACCOUNT_ID: string;
	CF_ZONE_ID: string;
	CF_DOMAIN: string;
}

interface AccessIdentityProvider {
	id: string;
	name: string;
	type: string;
}

interface AccessApplication {
	id?: string;
	name?: string;
	domain?: string;
	self_hosted_domains?: string[];
	destinations?: AccessDestination[];
}

interface AccessDestination {
	type?: string;
	uri?: string;
}

interface AccessPolicy {
	id?: string;
	decision?: string;
}

async function cfFetch(env: CFEnv, path: string, init?: RequestInit) {
	const res = await fetch(`${CF_API}${path}`, {
		...init,
		headers: {
			Authorization: `Bearer ${env.CF_API_TOKEN}`,
			'Content-Type': 'application/json',
			...init?.headers
		}
	});
	const data = await res.json() as { success: boolean; result: any; errors: any[] };
	if (!data.success) {
		throw new Error(`CF API error: ${JSON.stringify(data.errors)}`);
	}
	return data.result;
}

export async function createTunnel(env: CFEnv, deviceId: string) {
	const tunnelSecret = btoa(crypto.getRandomValues(new Uint8Array(32)).toString());

	const tunnel = await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/cfd_tunnel`, {
		method: 'POST',
		body: JSON.stringify({
			name: `internkim-${deviceId}`,
			tunnel_secret: tunnelSecret,
			config_src: 'cloudflare'
		})
	});

	return { tunnelId: tunnel.id as string, tunnelToken: tunnel.token as string };
}

export async function configureTunnel(env: CFEnv, tunnelId: string, deviceId: string) {
	const hostname = `${deviceId}.${env.CF_DOMAIN}`;

	await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/cfd_tunnel/${tunnelId}/configurations`, {
		method: 'PUT',
		body: JSON.stringify({
			config: {
				ingress: [
					{ hostname, service: 'http://127.0.0.1:18080' },
					{ service: 'http_status:404' }
				]
			}
		})
	});
}

export async function createDNSRecord(env: CFEnv, tunnelId: string, deviceId: string) {
	const record = await cfFetch(env, `/zones/${env.CF_ZONE_ID}/dns_records`, {
		method: 'POST',
		body: JSON.stringify({
			type: 'CNAME',
			name: `${deviceId}.${env.CF_DOMAIN}`,
			content: `${tunnelId}.cfargotunnel.com`,
			proxied: true
		})
	});

	return record.id as string;
}

export async function ensureOneTimePinIdentityProvider(env: CFEnv) {
	try {
		const identityProviders = (await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/access/identity_providers`)) as AccessIdentityProvider[];
		const existingProvider = identityProviders.find((identityProvider) => identityProvider.type === 'onetimepin');
		if (existingProvider) {
			return existingProvider.id;
		}

		const identityProvider = (await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/access/identity_providers`, {
			method: 'POST',
			body: JSON.stringify({
				name: 'One-time PIN',
				type: 'onetimepin',
				config: {}
			})
		})) as AccessIdentityProvider;

		return identityProvider.id;
	} catch (caughtError) {
		const message = caughtError instanceof Error ? caughtError.message : String(caughtError);
		throw new Error(
			`Cloudflare One-time PIN identity provider is not configured or cannot be managed by this API token: ${message}. ` +
				'Enable Zero Trust > Integrations > Identity providers > One-time PIN, or grant the token Access: Organizations, Identity Providers, and Groups Write.'
		);
	}
}

function accessApplicationBody(env: CFEnv, deviceId: string, identityProviderId: string) {
	const hostname = `${deviceId}.${env.CF_DOMAIN}`;

	return {
		name: 'intern kim',
		domain: hostname,
		type: 'self_hosted',
		session_duration: '720h',
		logo_url: `https://${hostname}/logo.svg`,
		allowed_idps: [identityProviderId],
		auto_redirect_to_identity: true
	};
}

function companionBypassApplicationBody(env: CFEnv, deviceId: string) {
	const hostname = `${deviceId}.${env.CF_DOMAIN}`;

	return {
		name: `intern kim companion ${deviceId}`,
		domain: `${hostname}/_internkim/companion/*`,
		type: 'self_hosted',
		session_duration: '1h'
	};
}

function adminAccessApplicationBody(env: CFEnv, deviceId: string, identityProviderId: string, domain: string) {
	const hostname = `${deviceId}.${env.CF_DOMAIN}`;

	return {
		name: `intern kim admin ${deviceId} ${domain}`,
		domain: `${hostname}${domain}`,
		type: 'self_hosted',
		session_duration: '720h',
		logo_url: `https://${hostname}/logo.svg`,
		allowed_idps: [identityProviderId],
		auto_redirect_to_identity: true
	};
}

function accessApplicationDomain(application: AccessApplication): string {
	if (typeof application.domain === 'string') return application.domain;
	if (Array.isArray(application.self_hosted_domains) && typeof application.self_hosted_domains[0] === 'string') {
		return application.self_hosted_domains[0];
	}
	if (Array.isArray(application.destinations)) {
		const destination = application.destinations.find((item) => item.type === 'public' && typeof item.uri === 'string');
		return destination?.uri ?? '';
	}
	return '';
}

export async function createAccessApplication(env: CFEnv, deviceId: string, identityProviderId: string) {
	const app = await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/access/apps`, {
		method: 'POST',
		body: JSON.stringify(accessApplicationBody(env, deviceId, identityProviderId))
	});

	return app.id as string;
}

export async function updateAccessApplicationLoginMethod(env: CFEnv, deviceId: string, appId: string, identityProviderId: string) {
	await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/access/apps/${appId}`, {
		method: 'PUT',
		body: JSON.stringify(accessApplicationBody(env, deviceId, identityProviderId))
	});
}

export async function ensureCompanionBypassApplication(env: CFEnv, deviceId: string) {
	const body = companionBypassApplicationBody(env, deviceId);
	const applications = (await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/access/apps`)) as AccessApplication[];
	const application = applications.find((item) => accessApplicationDomain(item) === body.domain);
	const applicationId = application?.id ?? await createCompanionBypassApplication(env, body);

	if (application?.id) {
		await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/access/apps/${applicationId}`, {
			method: 'PUT',
			body: JSON.stringify(body)
		});
	}

	await ensureCompanionBypassPolicy(env, applicationId);
	return applicationId;
}

export async function ensureAdminAccessApplications(env: CFEnv, deviceId: string, identityProviderId: string, adminEmails: string[] | string) {
	(void adminEmails);
	const applicationId = await ensureAdminAccessApplication(env, deviceId, identityProviderId, '/admin*');
	await syncAccessPolicyEveryone(env, applicationId);
	await deleteAccessApplicationForDomain(env, deviceId, '/_internkim/admin/*');
}

async function ensureAdminAccessApplication(env: CFEnv, deviceId: string, identityProviderId: string, domain: string) {
	const body = adminAccessApplicationBody(env, deviceId, identityProviderId, domain);
	const applications = (await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/access/apps`)) as AccessApplication[];
	const application = applications.find((item) => accessApplicationDomain(item) === body.domain);
	const applicationId = application?.id ?? await createAdminAccessApplication(env, body);

	if (application?.id) {
		await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/access/apps/${applicationId}`, {
			method: 'PUT',
			body: JSON.stringify(body)
		});
	}

	return applicationId;
}

async function createAdminAccessApplication(env: CFEnv, body: ReturnType<typeof adminAccessApplicationBody>) {
	const application = (await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/access/apps`, {
		method: 'POST',
		body: JSON.stringify(body)
	})) as AccessApplication;
	if (!application.id) {
		throw new Error('Cloudflare admin app response did not include an id');
	}
	return application.id;
}

async function deleteAccessApplicationForDomain(env: CFEnv, deviceId: string, domain: string) {
	const hostname = `${deviceId}.${env.CF_DOMAIN}`;
	const fullDomain = `${hostname}${domain}`;
	const applications = (await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/access/apps`)) as AccessApplication[];
	const application = applications.find((item) => accessApplicationDomain(item) === fullDomain);
	if (!application?.id) return;
	await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/access/apps/${application.id}`, {
		method: 'DELETE'
	});
}

async function createCompanionBypassApplication(env: CFEnv, body: ReturnType<typeof companionBypassApplicationBody>) {
	const application = (await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/access/apps`, {
		method: 'POST',
		body: JSON.stringify(body)
	})) as AccessApplication;
	if (!application.id) {
		throw new Error('Cloudflare companion bypass app response did not include an id');
	}
	return application.id;
}

async function ensureCompanionBypassPolicy(env: CFEnv, appId: string) {
	const policies = (await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/access/apps/${appId}/policies`)) as AccessPolicy[];
	const policy = policies.find((item) => item.decision === 'bypass');
	const body = {
		name: 'companion-pairing-and-broker',
		decision: 'bypass',
		include: [{ everyone: {} }]
	};

	if (!policy?.id) {
		await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/access/apps/${appId}/policies`, {
			method: 'POST',
			body: JSON.stringify(body)
		});
		return;
	}

	await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/access/apps/${appId}/policies/${policy.id}`, {
		method: 'PUT',
		body: JSON.stringify(body)
	});
}

export async function createAccessPolicy(env: CFEnv, appId: string, adminEmail: string) {
	if (!adminEmail.trim()) return createAccessPolicyForEveryone(env, appId);
	return createAccessPolicyForEmails(env, appId, [adminEmail]);
}

async function createAccessPolicyForEveryone(env: CFEnv, appId: string) {
	const policy = await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/access/apps/${appId}/policies`, {
		method: 'POST',
		body: JSON.stringify({
			name: 'allowed-users',
			decision: 'allow',
			include: [{ everyone: {} }]
		})
	});

	return policy.id as string;
}

async function createAccessPolicyForEmails(env: CFEnv, appId: string, emails: string[]) {
	const policy = await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/access/apps/${appId}/policies`, {
		method: 'POST',
		body: JSON.stringify({
			name: 'allowed-users',
			decision: 'allow',
			include: emails.map((email) => ({ email: { email } }))
		})
	});

	return policy.id as string;
}

export async function syncAccessPolicyEmails(env: CFEnv, appId: string, emails: string[]) {
	if (emails.length === 0) return syncAccessPolicyEveryone(env, appId);

	const policies = await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/access/apps/${appId}/policies`);
	const policy = policies[0];
	if (!policy) {
		return createAccessPolicyForEmails(env, appId, emails);
	}

	const include = emails.map((email) => ({ email: { email } }));

	await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/access/apps/${appId}/policies/${policy.id}`, {
		method: 'PUT',
		body: JSON.stringify({
			name: policy.name,
			decision: 'allow',
			include
		})
	});

	return policy.id as string;
}

async function syncAccessPolicyEveryone(env: CFEnv, appId: string) {
	const policies = await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/access/apps/${appId}/policies`);
	const policy = policies[0];
	if (!policy) return createAccessPolicyForEveryone(env, appId);

	await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/access/apps/${appId}/policies/${policy.id}`, {
		method: 'PUT',
		body: JSON.stringify({
			name: policy.name,
			decision: 'allow',
			include: [{ everyone: {} }]
		})
	});

	return policy.id as string;
}

export async function deleteTunnel(env: CFEnv, tunnelId: string) {
	await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/cfd_tunnel/${tunnelId}`, {
		method: 'DELETE'
	});
}

export async function deleteDNSRecord(env: CFEnv, recordId: string) {
	await cfFetch(env, `/zones/${env.CF_ZONE_ID}/dns_records/${recordId}`, {
		method: 'DELETE'
	});
}
