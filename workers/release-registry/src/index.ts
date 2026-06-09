import { handleReleaseRegistryRequest } from './registry';

export type ReleaseRegistryEnvironment = {
	RELEASE_BUCKET: R2Bucket;
	RELEASE_DOWNLOAD_TOKEN?: string;
};

export default {
	fetch(request: Request, environment: ReleaseRegistryEnvironment) {
		return handleReleaseRegistryRequest(request, environment);
	}
} satisfies ExportedHandler<ReleaseRegistryEnvironment>;
