alter table public.company_document
  add column clearance smallint not null default 1
    constraint company_document_clearance_is_zero_to_three check (clearance between 0 and 3),
  add column domain text
    constraint company_document_domain_is_named check (domain is null or btrim(domain) <> ''),
  add column document_date date,
  add column period text,
  add column status text
    constraint company_document_status_is_current_superseded_or_draft
    check (status is null or status in ('current', 'superseded', 'draft')),
  add column supersedes uuid,
  add column sha256 text
    constraint company_document_sha256_is_a_hex_digest check (sha256 ~ '^[0-9a-f]{64}$'),
  add column tags text[] not null default '{}',
  add column storage_path text,
  add column published_from uuid,
  add column published_at timestamptz,
  add column published_by uuid,
  add constraint company_document_company_id_id_key unique (company_id, id);

alter table public.company_document
  add constraint company_document_supersedes_belongs_to_company
    foreign key (company_id, supersedes)
    references public.company_document (company_id, id),
  add constraint company_document_published_from_belongs_to_company
    foreign key (company_id, published_from)
    references public.company_document (company_id, id),
  add constraint company_document_published_by_belongs_to_company
    foreign key (company_id, published_by)
    references public.member (company_id, id);

create index company_document_company_id_domain_clearance_idx
  on public.company_document (company_id, domain, clearance);

drop policy company_document_readable_by_colleague on public.company_document;
drop policy company_document_written_by_colleague on public.company_document;

create policy company_document_readable_by_a_cleared_colleague on public.company_document
  for select to authenticated
  using (
    company_id = internal.company_of_member(public.my_member())
    and clearance <= public.my_clearance()
  );

create policy company_document_written_by_a_cleared_colleague on public.company_document
  for all to authenticated
  using (
    company_id = internal.company_of_member(public.my_member())
    and clearance <= public.my_clearance()
  )
  with check (
    company_id = internal.company_of_member(public.my_member())
    and clearance <= public.my_clearance()
  );
