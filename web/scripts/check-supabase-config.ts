import localConfig from '../../supabase/config.toml';
import { projectReference, accessToken } from './remote-query';

type AuthConfig = Record<string, unknown>;
type PostgrestConfig = Record<string, unknown>;
type StorageConfig = {
	fileSizeLimit: number;
	features: {
		imageTransformation: { enabled: boolean };
		s3Protocol: { enabled: boolean };
		icebergCatalog: { enabled: boolean; maxNamespaces: number; maxTables: number; maxCatalogs: number };
		vectorBuckets: { enabled: boolean; maxBuckets: number; maxIndexes: number };
	};
};
type NetworkRestrictions = { config: { dbAllowedCidrs: string[]; dbAllowedCidrsV6: string[] } };
type SslEnforcement = { currentConfig: { database: boolean } };

async function managementGet<Response>(path: string): Promise<Response> {
	const response = await fetch(`https://api.supabase.com/v1/projects/${projectReference}/${path}`, {
		headers: { Authorization: `Bearer ${accessToken()}` }
	});
	const body = await response.text();
	if (!response.ok) throw new Error(`${response.status} ${body}`);
	return JSON.parse(body) as Response;
}

function bytesOfHumanSize(size: string): number {
	const match = size.match(/^(\d+(?:\.\d+)?)\s*(KiB|MiB|GiB)$/);
	if (!match) throw new Error(`unrecognized storage size: ${size}`);
	const magnitude = { KiB: 1024, MiB: 1024 ** 2, GiB: 1024 ** 3 }[match[2] as 'KiB' | 'MiB' | 'GiB'];
	return Number(match[1]) * magnitude;
}

type Comparison = { setting: string; local: unknown; live: unknown };

function compare(setting: string, local: unknown, live: unknown, comparisons: Comparison[]): void {
	comparisons.push({ setting, local, live });
}

const comparisons: Comparison[] = [];

const auth = await managementGet<AuthConfig>('config/auth');
const auth0 = localConfig.auth as Record<string, unknown>;
compare('auth.jwt_expiry', auth0.jwt_expiry, auth.jwt_exp, comparisons);
compare('auth.enable_signup', auth0.enable_signup, !auth.disable_signup, comparisons);
compare('auth.enable_anonymous_sign_ins', auth0.enable_anonymous_sign_ins, auth.external_anonymous_users_enabled, comparisons);
compare('auth.enable_manual_linking', auth0.enable_manual_linking, auth.security_manual_linking_enabled, comparisons);
compare('auth.minimum_password_length', auth0.minimum_password_length, auth.password_min_length, comparisons);
compare('auth.enable_refresh_token_rotation', auth0.enable_refresh_token_rotation, auth.refresh_token_rotation_enabled, comparisons);
compare('auth.refresh_token_reuse_interval', auth0.refresh_token_reuse_interval, auth.security_refresh_token_reuse_interval, comparisons);

const rateLimit = localConfig.auth.rate_limit;
compare('auth.rate_limit.anonymous_users', rateLimit.anonymous_users, auth.rate_limit_anonymous_users, comparisons);
compare('auth.rate_limit.token_refresh', rateLimit.token_refresh, auth.rate_limit_token_refresh, comparisons);
compare('auth.rate_limit.sign_in_sign_ups', rateLimit.sign_in_sign_ups, auth.rate_limit_otp, comparisons);
compare('auth.rate_limit.token_verifications', rateLimit.token_verifications, auth.rate_limit_verify, comparisons);
compare('auth.rate_limit.email_sent', rateLimit.email_sent, auth.rate_limit_email_sent, comparisons);
compare('auth.rate_limit.sms_sent', rateLimit.sms_sent, auth.rate_limit_sms_sent, comparisons);
compare('auth.rate_limit.web3', rateLimit.web3, auth.rate_limit_web3, comparisons);

const captcha = localConfig.auth.captcha;
compare('auth.captcha.enabled', captcha.enabled, auth.security_captcha_enabled, comparisons);

const mfa = localConfig.auth.mfa as Record<string, unknown>;
compare('auth.mfa.max_enrolled_factors', mfa.max_enrolled_factors, auth.mfa_max_enrolled_factors, comparisons);
const totp = localConfig.auth.mfa.totp;
compare('auth.mfa.totp.enroll_enabled', totp.enroll_enabled, auth.mfa_totp_enroll_enabled, comparisons);
compare('auth.mfa.totp.verify_enabled', totp.verify_enabled, auth.mfa_totp_verify_enabled, comparisons);
const phone = localConfig.auth.mfa.phone;
compare('auth.mfa.phone.enroll_enabled', phone.enroll_enabled, auth.mfa_phone_enroll_enabled, comparisons);
compare('auth.mfa.phone.verify_enabled', phone.verify_enabled, auth.mfa_phone_verify_enabled, comparisons);
const webAuthn = localConfig.auth.mfa.web_authn;
compare('auth.mfa.web_authn.enroll_enabled', webAuthn.enroll_enabled, auth.mfa_web_authn_enroll_enabled, comparisons);
compare('auth.mfa.web_authn.verify_enabled', webAuthn.verify_enabled, auth.mfa_web_authn_verify_enabled, comparisons);

const email = localConfig.auth.email as Record<string, unknown>;
compare('auth.email.enable_signup', email.enable_signup, auth.external_email_enabled, comparisons);
compare('auth.email.enable_confirmations', email.enable_confirmations, !auth.mailer_autoconfirm, comparisons);
compare('auth.email.double_confirm_changes', email.double_confirm_changes, auth.mailer_secure_email_change_enabled, comparisons);
compare(
	'auth.email.secure_password_change',
	email.secure_password_change,
	auth.security_update_password_require_reauthentication,
	comparisons
);
compare('auth.email.otp_length', email.otp_length, auth.mailer_otp_length, comparisons);
compare('auth.email.otp_expiry', email.otp_expiry, auth.mailer_otp_exp, comparisons);
compare('auth.email.smtp.enabled', localConfig.auth.email.smtp.enabled, Boolean(auth.smtp_host), comparisons);

const sms = localConfig.auth.sms as Record<string, unknown>;
compare('auth.sms.enable_signup', sms.enable_signup, auth.external_phone_enabled, comparisons);
compare('auth.sms.enable_confirmations', sms.enable_confirmations, !auth.sms_autoconfirm, comparisons);

const web3 = localConfig.auth.web3 as Record<string, { enabled: boolean }>;
compare('auth.web3.solana.enabled', web3.solana.enabled, auth.external_web3_solana_enabled, comparisons);
compare('auth.web3.ethereum.enabled', web3.ethereum.enabled, auth.external_web3_ethereum_enabled, comparisons);

const externalProviders = localConfig.auth.external as Record<string, { enabled: boolean }>;
for (const [provider, settings] of Object.entries(externalProviders)) {
	compare(`auth.external.${provider}.enabled`, settings.enabled, auth[`external_${provider}_enabled`], comparisons);
}

const hooks = localConfig.auth.hook as Record<string, { enabled: boolean }>;
for (const [hook, settings] of Object.entries(hooks)) {
	compare(`auth.hook.${hook}.enabled`, settings.enabled, auth[`hook_${hook}_enabled`], comparisons);
}

const oauthServer = localConfig.auth.oauth_server as Record<string, unknown>;
compare('auth.oauth_server.enabled', oauthServer.enabled, auth.oauth_server_enabled, comparisons);
compare(
	'auth.oauth_server.allow_dynamic_registration',
	oauthServer.allow_dynamic_registration,
	auth.oauth_server_allow_dynamic_registration,
	comparisons
);
compare(
	'auth.oauth_server.authorization_url_path',
	oauthServer.authorization_url_path,
	auth.oauth_server_authorization_path,
	comparisons
);

function commaSeparatedValues(value: string): string[] {
	return value.split(',').map((entry) => entry.trim());
}

const postgrest = await managementGet<PostgrestConfig>('postgrest');
const api = localConfig.api as Record<string, unknown>;
compare('api.schemas', api.schemas, commaSeparatedValues(postgrest.db_schema as string), comparisons);
compare(
	'api.extra_search_path',
	api.extra_search_path,
	commaSeparatedValues(postgrest.db_extra_search_path as string),
	comparisons
);
compare('api.max_rows', api.max_rows, postgrest.max_rows, comparisons);

const storage = await managementGet<StorageConfig>('config/storage');
const storageLocal = localConfig.storage as Record<string, unknown>;
compare('storage.file_size_limit', bytesOfHumanSize(storageLocal.file_size_limit as string), storage.fileSizeLimit, comparisons);
compare(
	'storage.image_transformation.enabled',
	localConfig.storage.image_transformation.enabled,
	storage.features.imageTransformation.enabled,
	comparisons
);
compare('storage.s3_protocol.enabled', localConfig.storage.s3_protocol.enabled, storage.features.s3Protocol.enabled, comparisons);
compare('storage.analytics.enabled', localConfig.storage.analytics.enabled, storage.features.icebergCatalog.enabled, comparisons);
compare(
	'storage.analytics.max_namespaces',
	localConfig.storage.analytics.max_namespaces,
	storage.features.icebergCatalog.maxNamespaces,
	comparisons
);
compare(
	'storage.analytics.max_tables',
	localConfig.storage.analytics.max_tables,
	storage.features.icebergCatalog.maxTables,
	comparisons
);
compare(
	'storage.analytics.max_catalogs',
	localConfig.storage.analytics.max_catalogs,
	storage.features.icebergCatalog.maxCatalogs,
	comparisons
);
compare('storage.vector.enabled', localConfig.storage.vector.enabled, storage.features.vectorBuckets.enabled, comparisons);
compare('storage.vector.max_buckets', localConfig.storage.vector.max_buckets, storage.features.vectorBuckets.maxBuckets, comparisons);
compare('storage.vector.max_indexes', localConfig.storage.vector.max_indexes, storage.features.vectorBuckets.maxIndexes, comparisons);

const networkRestrictions = await managementGet<NetworkRestrictions>('network-restrictions');
const localNetworkRestrictions = localConfig.db.network_restrictions as Record<string, unknown>;
compare(
	'db.network_restrictions.allowed_cidrs',
	localNetworkRestrictions.allowed_cidrs,
	networkRestrictions.config.dbAllowedCidrs,
	comparisons
);
compare(
	'db.network_restrictions.allowed_cidrs_v6',
	localNetworkRestrictions.allowed_cidrs_v6,
	networkRestrictions.config.dbAllowedCidrsV6,
	comparisons
);

const sslEnforcement = await managementGet<SslEnforcement>('ssl-enforcement');
compare(
	'db.ssl_enforcement.enabled',
	(localConfig.db.ssl_enforcement as Record<string, unknown>).enabled,
	sslEnforcement.currentConfig.database,
	comparisons
);

const mismatched = comparisons.filter(
	({ local, live }) => live !== undefined && JSON.stringify(local) !== JSON.stringify(live)
);

console.log(`project      ${projectReference}`);
console.log(`checked      ${comparisons.length} settings across auth, api, storage, db`);

if (mismatched.length === 0) {
	console.log('\nconfig.toml matches every hosted setting this script reads');
} else {
	for (const { setting, local, live } of mismatched) {
		console.log(`\n! config.toml does not match the hosted project: ${setting}`);
		console.log(`  config.toml: ${JSON.stringify(local)}`);
		console.log(`  hosted:      ${JSON.stringify(live)}`);
	}
}

if (mismatched.length > 0) process.exit(1);
