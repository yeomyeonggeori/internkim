const CF_API = 'https://api.cloudflare.com/client/v4';

export interface CFEnv {
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
	name?: string;
	decision?: string;
}

interface CertificatePack {
	id?: string;
	hosts?: string[];
	status?: string;
	type?: string;
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

export async function createTunnel(env: CFEnv, fleetId: string) {
	return createNamedTunnel(env, `internkim-${fleetId}`);
}

export async function createNodeSSHTunnel(env: CFEnv, fleetId: string, nodeId: string) {
	return createNamedTunnel(env, `internkim-${fleetId}-${nodeId}-ssh`);
}

async function createNamedTunnel(env: CFEnv, name: string) {
	const tunnelSecret = btoa(crypto.getRandomValues(new Uint8Array(32)).toString());

	const tunnel = await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/cfd_tunnel`, {
		method: 'POST',
		body: JSON.stringify({
			name,
			tunnel_secret: tunnelSecret,
			config_src: 'cloudflare'
		})
	});

	return { tunnelId: tunnel.id as string, tunnelToken: tunnel.token as string };
}

export async function configureTunnel(env: CFEnv, tunnelId: string, fleetId: string, aliasFleetIds: string[] = []) {
	const hostnames = [fleetId, ...aliasFleetIds]
		.map((value) => value.trim())
		.filter(Boolean)
		.map((value) => `${value}.${env.CF_DOMAIN}`);
	const ingress = hostnames.flatMap((hostname) => [
		{ hostname: `*.${hostname}`, service: 'http://127.0.0.1:18080' },
		{ hostname, service: 'http://127.0.0.1:18080' }
	]);

	await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/cfd_tunnel/${tunnelId}/configurations`, {
		method: 'PUT',
		body: JSON.stringify({
			config: {
				ingress: [
					...ingress,
					{ service: 'http_status:404' }
				]
			}
		})
	});
}

export async function configureNodeSSHTunnel(env: CFEnv, tunnelId: string, fleetId: string, nodeId: string, aliasFleetIds: string[] = []) {
	const ingress = [fleetId, ...aliasFleetIds]
		.map((value) => value.trim())
		.filter(Boolean)
		.map((value) => ({ hostname: nodeSSHHostname(env, value, nodeId), service: 'ssh://127.0.0.1:22' }));

	await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/cfd_tunnel/${tunnelId}/configurations`, {
		method: 'PUT',
		body: JSON.stringify({
			config: {
				ingress: [
					...ingress,
					{ service: 'http_status:404' }
				]
			}
		})
	});
}

export async function createDNSRecord(env: CFEnv, tunnelId: string, fleetId: string) {
	const record = await cfFetch(env, `/zones/${env.CF_ZONE_ID}/dns_records`, {
		method: 'POST',
		body: JSON.stringify({
			type: 'CNAME',
			name: `${fleetId}.${env.CF_DOMAIN}`,
			content: `${tunnelId}.cfargotunnel.com`,
			proxied: true
		})
	});

	return record.id as string;
}

export async function ensureFleetDNSRecord(env: CFEnv, tunnelId: string, fleetId: string) {
	const name = `${fleetId}.${env.CF_DOMAIN}`;
	return ensureDNSRecord(env, name, `${tunnelId}.cfargotunnel.com`);
}

export function fleetSSHHostname(env: CFEnv, fleetId: string) {
	return `ssh-${fleetId}.${env.CF_DOMAIN}`;
}

export function nodeSSHHostname(env: CFEnv, fleetId: string, nodeId: string) {
	return `${nodeId}.ssh.${fleetId}.${env.CF_DOMAIN}`;
}

export async function ensureSSHDNSRecord(env: CFEnv, tunnelId: string, fleetId: string) {
	return ensureDNSRecord(env, fleetSSHHostname(env, fleetId), `${tunnelId}.cfargotunnel.com`);
}

export async function ensureNodeSSHDNSRecord(env: CFEnv, tunnelId: string, fleetId: string, nodeId: string) {
	return ensureDNSRecord(env, nodeSSHHostname(env, fleetId, nodeId), `${tunnelId}.cfargotunnel.com`);
}

export async function ensureWildcardDNSRecord(env: CFEnv, tunnelId: string, fleetId: string) {
	const name = `*.${fleetId}.${env.CF_DOMAIN}`;
	return ensureDNSRecord(env, name, `${tunnelId}.cfargotunnel.com`);
}

export async function ensureFleetCertificateCoverage(env: CFEnv, fleetId: string, nodeId?: string) {
	const requiredHosts = certificateHostsForFleet(env, fleetId, nodeId);
	return ensureAdvancedCertificateHosts(env, requiredHosts);
}

async function ensureDNSRecord(env: CFEnv, name: string, content: string) {
	const records = (await cfFetch(env, `/zones/${env.CF_ZONE_ID}/dns_records?type=CNAME&name=${encodeURIComponent(name)}`)) as Array<{ id?: string }>;
	const existingRecord = records[0];
	if (existingRecord?.id) return existingRecord.id;
	const record = await cfFetch(env, `/zones/${env.CF_ZONE_ID}/dns_records`, {
		method: 'POST',
		body: JSON.stringify({
			type: 'CNAME',
			name,
			content,
			proxied: true
		})
	});

	return record.id as string;
}

async function ensureAdvancedCertificateHosts(env: CFEnv, requiredHosts: string[]) {
	try {
		const certificatePacks = (await cfFetch(env, `/zones/${env.CF_ZONE_ID}/ssl/certificate_packs?status=all`)) as CertificatePack[];
		const currentPacks = currentManagedCertificatePacks(env, certificatePacks);
		const missingHosts = requiredHosts.filter((host) => !currentPacks.some((pack) => certificatePackCoversHost(pack, host)));
		if (missingHosts.length === 0) {
			await cleanupCoveredCertificatePacks(env, certificatePacks);
			return { status: certificateCoverageStatus(currentPacks, requiredHosts), hosts: requiredHosts };
		}
		const hosts = certificatePackHostsForOrder(env, currentPacks, requiredHosts);
		const certificatePack = await orderAdvancedCertificatePack(env, hosts);
		return { status: certificatePack.status ?? 'ordered', hosts };
	} catch (caughtError) {
		const message = caughtError instanceof Error ? caughtError.message : String(caughtError);
		throw new Error(certificateCoverageErrorMessage(message));
	}
}

function certificateCoverageErrorMessage(message: string) {
	if (message.includes('"code":1450') || message.includes('Advanced Certificate Manager')) {
		return `Cloudflare Advanced Certificate Manager is not enabled for this zone: ${message}`;
	}
	return `Cloudflare TLS certificate coverage requires Advanced Certificate Manager and Zone SSL and Certificates Read/Write permission: ${message}`;
}

async function orderAdvancedCertificatePack(env: CFEnv, hosts: string[]): Promise<CertificatePack> {
	return await cfFetch(env, `/zones/${env.CF_ZONE_ID}/ssl/certificate_packs/order`, {
		method: 'POST',
		body: JSON.stringify({
			certificate_authority: 'lets_encrypt',
			hosts,
			type: 'advanced',
			validation_method: 'txt',
			validity_days: 90,
			cloudflare_branding: false
		})
	}) as CertificatePack;
}

function certificatePackHostsForOrder(env: CFEnv, packs: CertificatePack[], requiredHosts: string[]) {
	const baseHosts = certificateBaseHosts(env);
	const currentHosts = packs.flatMap((pack) => certificatePackHosts(pack))
		.filter((host) => isCurrentCertificateHost(env, host));
	const expandedHosts = uniqueCertificateHosts([
		...baseHosts,
		...currentHosts.filter((host) => !baseHosts.includes(host)),
		...requiredHosts
	]);
	if (expandedHosts.length <= 50) return expandedHosts;
	const minimalHosts = uniqueCertificateHosts([
		...baseHosts,
		...requiredHosts
	]);
	if (minimalHosts.length > 50) {
		throw new Error('Cloudflare advanced certificate host limit reached; use a new fleet ID namespace or single-label hostnames');
	}
	return minimalHosts;
}

function certificateHostsForFleet(env: CFEnv, fleetId: string, nodeId?: string) {
	return [
		`*.${fleetId}.${env.CF_DOMAIN}`,
		...(nodeId ? [`*.ssh.${fleetId}.${env.CF_DOMAIN}`] : [])
	];
}

function certificatePackCoversHost(pack: CertificatePack, host: string) {
	if (!isReusableCertificatePack(pack)) return false;
	return (pack.hosts ?? []).some((pattern) => certificateHostMatches(pattern, host));
}

async function cleanupCoveredCertificatePacks(env: CFEnv, packs: CertificatePack[]) {
	const coveringPack = currentManagedCertificatePacks(env, packs)
		.filter((pack) => pack.status === 'active')
		.slice()
		.sort((leftPack, rightPack) => currentCertificateHostCount(env, rightPack) - currentCertificateHostCount(env, leftPack))[0];
	if (!coveringPack?.id) return;
	const cleanupPacks = internKimCertificatePacks(env, packs)
		.filter((pack) => pack.status !== 'deleted' && pack.status !== 'pending_deletion');
	for (const pack of cleanupPacks) {
		if (!pack.id || pack.id === coveringPack.id) continue;
		if (certificatePackCanBeReplaced(env, pack, coveringPack)) {
			await cfFetch(env, `/zones/${env.CF_ZONE_ID}/ssl/certificate_packs/${pack.id}`, { method: 'DELETE' });
		}
	}
}

function currentCertificateHostCount(env: CFEnv, pack: CertificatePack) {
	return certificatePackHosts(pack).filter((host) => isCurrentCertificateHost(env, host)).length;
}

function certificatePackCanBeReplaced(env: CFEnv, pack: CertificatePack, coveringPack: CertificatePack) {
	const hosts = certificatePackHosts(pack);
	const coveringHosts = certificatePackHosts(coveringPack);
	return hosts.every((host) => coveringHosts.includes(host) || isLegacyGeneratedCertificateHost(env, host));
}

function currentManagedCertificatePacks(env: CFEnv, packs: CertificatePack[]) {
	return packs.filter((pack) =>
		isReusableCertificatePack(pack) &&
		certificateBaseHosts(env).every((host) => certificatePackHosts(pack).includes(host)) &&
		certificatePackHosts(pack).every((host) => isCurrentCertificateHost(env, host))
	);
}

function internKimCertificatePacks(env: CFEnv, packs: CertificatePack[]) {
	return packs.filter((pack) =>
		pack.type === 'advanced' &&
		certificateBaseHosts(env).every((host) => certificatePackHosts(pack).includes(host)) &&
		certificatePackHosts(pack).every((host) => isCurrentCertificateHost(env, host) || isLegacyGeneratedCertificateHost(env, host))
	);
}

function certificateBaseHosts(env: CFEnv) {
	return [env.CF_DOMAIN, `*.${env.CF_DOMAIN}`].map((host) => host.toLowerCase());
}

function certificatePackHosts(pack: CertificatePack) {
	return uniqueCertificateHosts(pack.hosts ?? []);
}

function uniqueCertificateHosts(hosts: string[]) {
	return [...new Set(hosts.map((host) => host.trim().toLowerCase()).filter(Boolean))];
}

function isGeneratedCertificateHost(env: CFEnv, host: string) {
	const labels = host.toLowerCase().split('.');
	const domainLabels = env.CF_DOMAIN.toLowerCase().split('.');
	if (labels[0] !== '*') return false;
	if (labels.length === domainLabels.length + 2) {
		return labels.slice(-domainLabels.length).join('.') === env.CF_DOMAIN.toLowerCase();
	}
	if (labels.length !== domainLabels.length + 3) return false;
	if (labels[1] !== 'ssh') return false;
	return labels.slice(-domainLabels.length).join('.') === env.CF_DOMAIN.toLowerCase();
}

function isCurrentCertificateHost(env: CFEnv, host: string) {
	return certificateBaseHosts(env).includes(host) || isGeneratedCertificateHost(env, host);
}

function isLegacyGeneratedCertificateHost(env: CFEnv, host: string) {
	const labels = host.toLowerCase().split('.');
	const domainLabels = env.CF_DOMAIN.toLowerCase().split('.');
	if (labels.length !== domainLabels.length + 3) return false;
	if (labels[0] !== '*') return false;
	if (labels[1] === 'ssh') return false;
	return labels.slice(-domainLabels.length).join('.') === env.CF_DOMAIN.toLowerCase();
}

function certificateCoverageStatus(packs: CertificatePack[], hosts: string[]) {
	if (hosts.every((host) => packs.some((pack) => certificatePackActivelyCoversHost(pack, host)))) {
		return 'active';
	}
	const coveringPack = packs.find((pack) => hosts.some((host) => certificatePackCoversHost(pack, host)));
	return coveringPack?.status ?? 'pending';
}

function certificatePackActivelyCoversHost(pack: CertificatePack, host: string) {
	if (pack.type !== 'advanced' || pack.status !== 'active') return false;
	return (pack.hosts ?? []).some((pattern) => certificateHostMatches(pattern, host));
}

function isReusableCertificatePack(pack: CertificatePack) {
	return pack.type === 'advanced' && ![
		'deleted',
		'inactive',
		'expired',
		'deactivating',
		'pending_deletion',
		'initializing_timed_out',
		'validation_timed_out',
		'issuance_timed_out',
		'deployment_timed_out',
		'deletion_timed_out'
	].includes(pack.status ?? '');
}

function certificateHostMatches(pattern: string, host: string) {
	const normalizedPattern = pattern.trim().toLowerCase();
	const normalizedHost = host.trim().toLowerCase();
	if (normalizedPattern === normalizedHost) return true;
	if (!normalizedPattern.startsWith('*.')) return false;
	const patternLabels = normalizedPattern.split('.');
	const hostLabels = normalizedHost.split('.');
	return patternLabels.length === hostLabels.length &&
		patternLabels.slice(1).join('.') === hostLabels.slice(1).join('.');
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

function companionBypassApplicationBody(env: CFEnv, fleetId: string) {
	const hostname = `${fleetId}.${env.CF_DOMAIN}`;

	return {
		name: `intern kim companion ${fleetId}`,
		domain: `${hostname}/_internkim/companion/*`,
		type: 'self_hosted',
		session_duration: '1h'
	};
}

function maintenanceBypassApplicationBody(env: CFEnv, fleetId: string) {
	const hostname = `${fleetId}.${env.CF_DOMAIN}`;

	return {
		name: `intern kim maintenance ${fleetId}`,
		self_hosted_domains: [
			`${hostname}/admin/api/health`,
			`${hostname}/admin/api/recovery/ssh-tunnel/restart`,
			`${hostname}/admin/api/updates/blueclaw/uploads`,
			`${hostname}/admin/api/updates/blueclaw/uploads/*`
		],
		type: 'self_hosted',
		session_duration: '1h'
	};
}

function releaseUpdateBypassApplicationBody(env: CFEnv, fleetId: string) {
	const hostname = `${fleetId}.${env.CF_DOMAIN}`;

	return {
		name: `intern kim release updates ${fleetId}`,
		self_hosted_domains: [
			`${hostname}/admin/api/updates/apply`,
			`${hostname}/admin/api/updates/status`,
			`${hostname}/admin/api/updates/jobs/*`
		],
		type: 'self_hosted',
		session_duration: '1h'
	};
}

function releaseUploadBypassApplicationBody(env: CFEnv, fleetId: string) {
	const hostname = `${fleetId}.${env.CF_DOMAIN}`;

	return {
		name: `intern kim release uploads ${fleetId}`,
		self_hosted_domains: [
			`${hostname}/admin/api/updates/uploads`,
			`${hostname}/admin/api/updates/uploads/*`
		],
		type: 'self_hosted',
		session_duration: '1h'
	};
}

function adminAccessApplicationBody(env: CFEnv, fleetId: string, identityProviderId: string) {
	const hostname = `${fleetId}.${env.CF_DOMAIN}`;

	return {
		name: `intern kim admin ${fleetId}`,
		self_hosted_domains: adminAccessApplicationDomains(env, fleetId),
		type: 'self_hosted',
		session_duration: '720h',
		logo_url: `https://${hostname}/logo.svg`,
		allowed_idps: [identityProviderId],
		auto_redirect_to_identity: true
	};
}

export function adminAccessApplicationDomains(env: CFEnv, fleetId: string) {
	const hostname = `${fleetId}.${env.CF_DOMAIN}`;
	return [`${hostname}/admin*`];
}

function webSessionAccessApplicationBody(env: CFEnv, fleetId: string, identityProviderId: string) {
	const hostname = `${fleetId}.${env.CF_DOMAIN}`;

	return {
		name: `intern kim web session ${fleetId}`,
		self_hosted_domains: webSessionAccessApplicationDomains(env, fleetId),
		type: 'self_hosted',
		session_duration: '720h',
		logo_url: `https://${hostname}/logo.svg`,
		allowed_idps: [identityProviderId],
		auto_redirect_to_identity: true
	};
}

export function webSessionAccessApplicationDomains(env: CFEnv, fleetId: string) {
	const hostname = `${fleetId}.${env.CF_DOMAIN}`;
	return [`${hostname}/auth/verify/*`];
}

function sshAccessApplicationBody(env: CFEnv, fleetId: string, identityProviderId: string) {
	return sshAccessApplicationBodyForHostname(env, fleetId, fleetSSHHostname(env, fleetId), identityProviderId);
}

function nodeSSHAccessApplicationBody(env: CFEnv, fleetId: string, nodeId: string, identityProviderId: string) {
	return sshAccessApplicationBodyForHostname(env, fleetId, nodeSSHHostname(env, fleetId, nodeId), identityProviderId);
}

function sshAccessApplicationBodyForHostname(env: CFEnv, fleetId: string, hostname: string, identityProviderId: string) {
	return {
		name: `intern kim ssh ${hostname}`,
		domain: hostname,
		type: 'self_hosted',
		session_duration: '24h',
		logo_url: `https://${fleetId}.${env.CF_DOMAIN}/logo.svg`,
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

export async function ensureCompanionBypassApplication(env: CFEnv, fleetId: string) {
	const body = companionBypassApplicationBody(env, fleetId);
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

export async function ensureMaintenanceBypassApplication(env: CFEnv, fleetId: string) {
	const body = maintenanceBypassApplicationBody(env, fleetId);
	const applications = (await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/access/apps`)) as AccessApplication[];
	const primaryDomain = body.self_hosted_domains[0];
	const application = applications.find((item) => accessApplicationDomain(item) === primaryDomain);
	const applicationId = application?.id ?? await createMaintenanceBypassApplication(env, body);

	if (application?.id) {
		await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/access/apps/${applicationId}`, {
			method: 'PUT',
			body: JSON.stringify(body)
		});
	}

	await ensureBypassPolicy(env, applicationId, 'maintenance-health-and-recovery');
	return applicationId;
}

export async function ensureReleaseUpdateBypassApplication(env: CFEnv, fleetId: string) {
	const body = releaseUpdateBypassApplicationBody(env, fleetId);
	const applications = (await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/access/apps`)) as AccessApplication[];
	const primaryDomain = body.self_hosted_domains[0];
	const application = applications.find((item) => accessApplicationDomain(item) === primaryDomain);
	const applicationId = application?.id ?? await createReleaseUpdateBypassApplication(env, body);

	if (application?.id) {
		await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/access/apps/${applicationId}`, {
			method: 'PUT',
			body: JSON.stringify(body)
		});
	}

	await ensureBypassPolicy(env, applicationId, 'release-update-signed-api');
	return applicationId;
}

export async function ensureReleaseUploadBypassApplication(env: CFEnv, fleetId: string) {
	const body = releaseUploadBypassApplicationBody(env, fleetId);
	const applications = (await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/access/apps`)) as AccessApplication[];
	const primaryDomain = body.self_hosted_domains[0];
	const application = applications.find((item) => accessApplicationDomain(item) === primaryDomain);
	const applicationId = application?.id ?? await createReleaseUploadBypassApplication(env, body);

	if (application?.id) {
		await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/access/apps/${applicationId}`, {
			method: 'PUT',
			body: JSON.stringify(body)
		});
	}

	await ensureBypassPolicy(env, applicationId, 'release-upload-signed-api');
	return applicationId;
}

export async function ensureAdminAccessApplications(env: CFEnv, fleetId: string, identityProviderId: string, adminEmails: string[] | string) {
	await deleteLegacyNonAdminAccessApplications(env, fleetId);
	const applicationID = await ensureAdminAccessApplication(env, fleetId, identityProviderId);
	await syncAccessPolicyEmails(env, applicationID, normalizeRequiredAccessEmails(adminEmails, 'Cloudflare admin access'));
}

export async function ensureWebSessionAccessApplication(env: CFEnv, fleetId: string, identityProviderId: string, emails: string[] | string) {
	const body = webSessionAccessApplicationBody(env, fleetId, identityProviderId);
	const applications = (await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/access/apps`)) as AccessApplication[];
	const primaryDomain = body.self_hosted_domains[0];
	const application = applications.find((item) => accessApplicationDomain(item) === primaryDomain);
	const applicationID = application?.id ?? await createWebSessionAccessApplication(env, body);

	if (application?.id) {
		await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/access/apps/${applicationID}`, {
			method: 'PUT',
			body: JSON.stringify(body)
		});
	}

	await syncAccessPolicyEmails(env, applicationID, normalizeRequiredAccessEmails(emails, 'Cloudflare web session access'));
	return applicationID;
}

export async function deleteRootAccessApplication(env: CFEnv, fleetId: string, applicationId?: string) {
	const hostname = `${fleetId}.${env.CF_DOMAIN}`;
	const applications = (await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/access/apps`)) as AccessApplication[];
	const applicationIDs = [...new Set(applications
		.filter((application) => application.id && (application.id === applicationId || accessApplicationDomain(application) === hostname))
		.map((application) => application.id as string))];

	await Promise.all(applicationIDs.map((id) => deleteAccessApplication(env, id)));
}

export async function ensureSSHAccessApplication(env: CFEnv, fleetId: string, identityProviderId: string, emails: string[] | string) {
	const body = sshAccessApplicationBody(env, fleetId, identityProviderId);
	return ensureSSHAccessApplicationForBody(env, body, emails);
}

export async function ensureNodeSSHAccessApplication(env: CFEnv, fleetId: string, nodeId: string, identityProviderId: string, emails: string[] | string) {
	const body = nodeSSHAccessApplicationBody(env, fleetId, nodeId, identityProviderId);
	return ensureSSHAccessApplicationForBody(env, body, emails);
}

async function ensureSSHAccessApplicationForBody(env: CFEnv, body: ReturnType<typeof sshAccessApplicationBody>, emails: string[] | string) {
	const applications = (await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/access/apps`)) as AccessApplication[];
	const application = applications.find((item) => accessApplicationDomain(item) === body.domain);
	const applicationId = application?.id ?? await createSSHAccessApplication(env, body);

	if (application?.id) {
		await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/access/apps/${applicationId}`, {
			method: 'PUT',
			body: JSON.stringify(body)
		});
	}

	await syncAccessPolicyEmails(env, applicationId, normalizeRequiredAccessEmails(emails, 'Cloudflare SSH access'));
	return applicationId;
}

async function ensureAdminAccessApplication(env: CFEnv, fleetId: string, identityProviderId: string) {
	const body = adminAccessApplicationBody(env, fleetId, identityProviderId);
	const applications = (await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/access/apps`)) as AccessApplication[];
	const primaryDomain = body.self_hosted_domains[0];
	const application = applications.find((item) => accessApplicationDomain(item) === primaryDomain);
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

async function createWebSessionAccessApplication(env: CFEnv, body: ReturnType<typeof webSessionAccessApplicationBody>) {
	const application = (await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/access/apps`, {
		method: 'POST',
		body: JSON.stringify(body)
	})) as AccessApplication;
	if (!application.id) {
		throw new Error('Cloudflare web session app response did not include an id');
	}
	return application.id;
}

async function deleteAccessApplication(env: CFEnv, applicationId: string) {
	await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/access/apps/${applicationId}`, {
		method: 'DELETE'
	});
}

async function createSSHAccessApplication(env: CFEnv, body: ReturnType<typeof sshAccessApplicationBody>) {
	const application = (await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/access/apps`, {
		method: 'POST',
		body: JSON.stringify(body)
	})) as AccessApplication;
	if (!application.id) {
		throw new Error('Cloudflare SSH app response did not include an id');
	}
	return application.id;
}

function normalizeAccessEmails(emails: string[] | string) {
	const values = Array.isArray(emails) ? emails : [emails];
	return values.map((email) => email.trim().toLowerCase()).filter(Boolean);
}

function normalizeRequiredAccessEmails(emails: string[] | string, label: string) {
	const values = normalizeAccessEmails(emails);
	if (values.length === 0) {
		throw new Error(`${label} requires at least one admin email`);
	}
	return values;
}

async function deleteAccessApplicationForDomain(env: CFEnv, fleetId: string, domain: string) {
	const hostname = `${fleetId}.${env.CF_DOMAIN}`;
	const fullDomain = `${hostname}${domain}`;
	const applications = (await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/access/apps`)) as AccessApplication[];
	const application = applications.find((item) => accessApplicationDomain(item) === fullDomain);
	if (!application?.id) return;
	await deleteAccessApplication(env, application.id);
}

async function deleteLegacyNonAdminAccessApplications(env: CFEnv, fleetId: string) {
	await Promise.all([
		deleteAccessApplicationForDomain(env, fleetId, '/_app/*'),
		deleteAccessApplicationForDomain(env, fleetId, '/flow*'),
		deleteAccessApplicationForDomain(env, fleetId, '/mail*'),
		deleteAccessApplicationForDomain(env, fleetId, '/calendar'),
		deleteAccessApplicationForDomain(env, fleetId, '/calendar/ics/*'),
		deleteAccessApplicationForDomain(env, fleetId, '/_internkim/admin/*')
	]);
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

async function createMaintenanceBypassApplication(env: CFEnv, body: ReturnType<typeof maintenanceBypassApplicationBody>) {
	const application = (await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/access/apps`, {
		method: 'POST',
		body: JSON.stringify(body)
	})) as AccessApplication;
	if (!application.id) {
		throw new Error('Cloudflare maintenance bypass app response did not include an id');
	}
	return application.id;
}

async function createReleaseUpdateBypassApplication(env: CFEnv, body: ReturnType<typeof releaseUpdateBypassApplicationBody>) {
	const application = (await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/access/apps`, {
		method: 'POST',
		body: JSON.stringify(body)
	})) as AccessApplication;
	if (!application.id) {
		throw new Error('Cloudflare release update bypass app response did not include an id');
	}
	return application.id;
}

async function createReleaseUploadBypassApplication(env: CFEnv, body: ReturnType<typeof releaseUploadBypassApplicationBody>) {
	const application = (await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/access/apps`, {
		method: 'POST',
		body: JSON.stringify(body)
	})) as AccessApplication;
	if (!application.id) {
		throw new Error('Cloudflare release upload bypass app response did not include an id');
	}
	return application.id;
}

async function ensureCompanionBypassPolicy(env: CFEnv, appId: string) {
	await ensureBypassPolicy(env, appId, 'companion-pairing-and-broker');
}

async function ensureBypassPolicy(env: CFEnv, appId: string, name: string) {
	const policies = (await cfFetch(env, `/accounts/${env.CF_ACCOUNT_ID}/access/apps/${appId}/policies`)) as AccessPolicy[];
	const policy = policies.find((item) => item.decision === 'bypass');
	const body = {
		name,
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

export async function syncSSHAccessPolicyEmails(env: CFEnv, appId: string, emails: string[] | string) {
	return syncAccessPolicyEmails(env, appId, normalizeRequiredAccessEmails(emails, 'Cloudflare SSH access'));
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
