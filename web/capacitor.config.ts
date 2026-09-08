import type { CapacitorConfig } from '@capacitor/cli';

const config: CapacitorConfig = {
	appId: 'kim.intern.app',
	appName: 'internkim',
	webDir: 'app-shell',
	server: { url: 'https://intern.kim' },
	ios: { contentInset: 'always' }
};

export default config;
