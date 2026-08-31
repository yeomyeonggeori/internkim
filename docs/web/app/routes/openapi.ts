import { createOpenApiDocument, type ApiDocumentationLanguage } from '../lib/openapi';

const languages: readonly ApiDocumentationLanguage[] = ['en', 'ko'];

export function loader({ params }: { params: { language?: string } }) {
  const language = languages.find((known) => known === params.language) ?? null;
  if (!language) return new Response('OpenAPI document not found', { status: 404 });
  return Response.json(createOpenApiDocument(language), {
    headers: { 'Cache-Control': 'public, max-age=0, s-maxage=3600' },
  });
}
