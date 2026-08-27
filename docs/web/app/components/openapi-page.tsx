import { createOpenAPIPage, type OpenAPIPageProps_Preloaded } from 'fumadocs-openapi/ui';
import generated from '@/generated/openapi.json';

// fumadocs-openapi types `Document.openapi` as the literal '3.2.0', which no
// JSON import can satisfy; its loader is what stamped that version on ours.
const documents = generated as unknown as OpenAPIPageProps_Preloaded['preloaded']['docs'];

const GeneratedPage = createOpenAPIPage();

export function OpenAPIPage(props: Omit<OpenAPIPageProps_Preloaded, 'preloaded'>) {
  return <GeneratedPage {...props} preloaded={{ docs: documents }} />;
}
