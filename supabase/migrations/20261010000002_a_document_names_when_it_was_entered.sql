drop function public.created_at(public.company_document);

alter table public.company_document rename column issued_at to created_at;
alter index public.company_document_company_id_issued_at_idx rename to company_document_company_id_created_at_idx;

create or replace function public.company_documents_matching(target_query text, target_limit integer)
returns setof public.company_document
language sql
security invoker
stable
set search_path = public, extensions
as $$
  select document.*
  from public.company_document as document
  where greatest(
      similarity(coalesce(document.title, ''), target_query),
      similarity(coalesce(document.counterpart, ''), target_query),
      similarity(coalesce(document.summary, ''), target_query),
      similarity(coalesce(document.document_type, ''), target_query)
    ) > 0
  order by greatest(
      similarity(coalesce(document.title, ''), target_query),
      similarity(coalesce(document.counterpart, ''), target_query),
      similarity(coalesce(document.summary, ''), target_query),
      similarity(coalesce(document.document_type, ''), target_query)
    ) desc,
    document.created_at desc
  limit greatest(1, least(coalesce(target_limit, 5), 50));
$$;

comment on column public.company_document.created_at is
  'when the document was entered in the data room; the day the document itself bears is document_date';
