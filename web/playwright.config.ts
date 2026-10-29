import { execFileSync } from 'node:child_process';
import { defineConfig, devices } from '@playwright/test';

const companyTimeZone = 'Asia/Seoul';
process.env.TZ = companyTimeZone;

type Plane = 'central' | 'fixture' | 'mocked' | 'standalone';

const namedPlanes: Exclude<Plane, 'standalone'>[] = ['central', 'fixture', 'mocked'];
const planes: Plane[] = [...namedPlanes, 'standalone'];

const withoutCentralPlane = { SUPABASE_URL: '', SUPABASE_PUBLISHABLE_KEY: '', SUPABASE_SECRET_KEY: '' };

const planeEnvironments: Record<Plane, Record<string, string>> = {
	central: {
		SUPABASE_URL: process.env.SUPABASE_URL ?? '',
		SUPABASE_PUBLISHABLE_KEY: process.env.SUPABASE_PUBLISHABLE_KEY ?? '',
		SUPABASE_SECRET_KEY: process.env.SUPABASE_SECRET_KEY ?? ''
	},
	fixture: {
		SUPABASE_URL: 'http://127.0.0.1:56801',
		SUPABASE_PUBLISHABLE_KEY: 'nonfunctional-fixture-key',
		SUPABASE_SECRET_KEY: '',
		GATEWAY_URL: 'http://127.0.0.1:56802'
	},
	mocked: {
		...withoutCentralPlane,
		VITE_MOCK_ADMIN: '1',
		VITE_MOCK_CRM: '1',
		VITE_MOCK_FILES: '1',
		VITE_MOCK_MAIL: '1',
		VITE_MOCK_MEMORY: '1',
		VITE_MOCK_TASKS: '1',
		VITE_ATTENDANCE_PREVIEW: '1'
	},
	standalone: withoutCentralPlane
};

const freePortsProgram = `
const servers = Array.from({ length: ${planes.length} }, () => Bun.listen({ hostname: '127.0.0.1', port: 0, socket: { data() {} } }));
console.log(servers.map((server) => server.port).join(','));
for (const server of servers) server.stop(true);
`;

process.env.E2E_SERVER_PORTS ??= execFileSync('bun', ['-e', freePortsProgram], { encoding: 'utf8' }).trim();
const serverPorts = process.env.E2E_SERVER_PORTS.split(',');

function baseURLOf(plane: Plane): string {
	return `http://127.0.0.1:${serverPorts[planes.indexOf(plane)]}`;
}

function specsOn(plane: Exclude<Plane, 'standalone'>): RegExp {
	return new RegExp(`-${plane}\\.spec\\.ts$`);
}

function devServerFor(plane: Plane) {
	const url = baseURLOf(plane);
	return {
		command: `bun run dev --host 127.0.0.1 --port ${new URL(url).port} --strictPort`,
		url,
		timeout: 120_000,
		reuseExistingServer: false,
		env: { VITE_ADMIND_TARGET: '', ...planeEnvironments[plane] }
	};
}

export default defineConfig({
	testDir: './tests/e2e',
	workers: '30%',
	timeout: 30_000,
	expect: { timeout: 10_000 },
	use: {
		...devices['Desktop Chrome'],
		timezoneId: companyTimeZone,
		trace: 'retain-on-failure'
	},
	projects: [
		...namedPlanes.map((plane) => ({
			name: plane,
			testMatch: specsOn(plane),
			workers: plane === 'central' ? 1 : undefined,
			use: { baseURL: baseURLOf(plane) }
		})),
		{ name: 'standalone', testIgnore: namedPlanes.map(specsOn), use: { baseURL: baseURLOf('standalone') } }
	],
	webServer: planes.map(devServerFor),
	reporter: [['list']]
});
