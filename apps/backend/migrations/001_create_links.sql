create table if not exists links (
    id bigserial primary key,
    slug varchar(32) unique not null,
    target_url text not null,
    created_at timestamptz not null default now()
);

create index if not exists idx_links_slug on links (slug);
