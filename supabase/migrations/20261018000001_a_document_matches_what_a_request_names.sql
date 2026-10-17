drop function public.company_documents_matching(text, integer);

create function public.company_documents_matching(
  target_query text,
  target_limit integer,
  minimum_relevance real default 0
)
returns setof public.company_document
language sql
security invoker
stable
set search_path = public, extensions
as $$
  select document.*
  from public.company_document as document
  cross join lateral (
    select greatest(
      word_similarity(target_query, coalesce(document.title, '')),
      word_similarity(coalesce(document.title, ''), target_query),
      word_similarity(target_query, coalesce(document.counterpart, '')),
      word_similarity(coalesce(document.counterpart, ''), target_query),
      word_similarity(target_query, coalesce(document.summary, ''))
    ) as relevance
  ) as matched
  where matched.relevance > 0
    and matched.relevance >= coalesce(minimum_relevance, 0)
  order by matched.relevance desc, document.created_at desc
  limit greatest(1, least(coalesce(target_limit, 5), 50));
$$;

revoke execute on function public.company_documents_matching(text, integer, real)
  from public, anon, service_role;
grant execute on function public.company_documents_matching(text, integer, real) to authenticated;
