import defaultMdxComponents from 'fumadocs-ui/mdx';
import { Step, Steps } from 'fumadocs-ui/components/steps';
import { Tab, Tabs } from 'fumadocs-ui/components/tabs';
import { Accordion, Accordions } from 'fumadocs-ui/components/accordion';
import { File, Files, Folder } from 'fumadocs-ui/components/files';
import { TypeTable } from 'fumadocs-ui/components/type-table';
import type { MDXComponents } from 'mdx/types';
import { ToolCatalog, ToolCount, ToolProtocolVersion } from './tool-catalog';
import { Mermaid } from './mermaid';
import { OpenAPIPage } from './openapi-page';
import { ToolReference } from './tool-reference';

export function getMDXComponents(components?: MDXComponents) {
  return {
    ...defaultMdxComponents,
    Steps,
    Step,
    Tabs,
    Tab,
    Accordions,
    Accordion,
    Files,
    Folder,
    File,
    TypeTable,
    Mermaid,
    ToolCatalog,
    ToolCount,
    ToolProtocolVersion,
    OpenAPIPage,
    ToolReference,
    ...components,
  } satisfies MDXComponents;
}

export const useMDXComponents = getMDXComponents;

declare global {
  type MDXProvidedComponents = ReturnType<typeof getMDXComponents>;
}
