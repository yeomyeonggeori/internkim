alter table public.empty_box
  add column host_name text check (host_name ~ '^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$'),
  add column pairing_page_addresses text[] not null default '{}'
    check (cardinality(pairing_page_addresses) <= 2);

comment on column public.empty_box.pairing_page_addresses is
  'where on its own network the box shows its pairing code, as the box reported it';
