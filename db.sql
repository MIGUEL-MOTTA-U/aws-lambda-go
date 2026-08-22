-- =============================================================================
-- Database Schema for Real Estate Backend (Neon PostgreSQL / Supabase)
-- Single Source of Truth for Database Tables & Indexes
-- =============================================================================

-- 1. Table: users
CREATE TABLE IF NOT EXISTS users (
    id VARCHAR(255) PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT NOT NULL,
    username TEXT NOT NULL,
    birthdate TEXT NOT NULL,
    creationdate TEXT NOT NULL,
    metadata JSONB DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);

-- 2. Table: listings
CREATE TABLE IF NOT EXISTS listings (
    listing_id VARCHAR(255) PRIMARY KEY,
    external_id TEXT,
    slug TEXT NOT NULL,
    url TEXT NOT NULL,
    language VARCHAR(10) NOT NULL DEFAULT 'es',
    title TEXT NOT NULL,
    description_short TEXT,
    description_long TEXT,
    property_type VARCHAR(100) NOT NULL,
    subtype VARCHAR(100),
    classification VARCHAR(100),
    operation_type VARCHAR(50) NOT NULL,
    publication_status VARCHAR(50) NOT NULL,
    featured BOOLEAN DEFAULT false,
    location JSONB NOT NULL DEFAULT '{}'::jsonb,
    pricing JSONB NOT NULL DEFAULT '{}'::jsonb,
    areas JSONB NOT NULL DEFAULT '{}'::jsonb,
    layout JSONB NOT NULL DEFAULT '{}'::jsonb,
    structure JSONB NOT NULL DEFAULT '{}'::jsonb,
    features JSONB NOT NULL DEFAULT '{}'::jsonb,
    media JSONB NOT NULL DEFAULT '{}'::jsonb,
    commercial JSONB NOT NULL DEFAULT '{}'::jsonb,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_listings_slug ON listings(slug);
CREATE INDEX IF NOT EXISTS idx_listings_publication_status ON listings(publication_status);
CREATE INDEX IF NOT EXISTS idx_listings_operation_type ON listings(operation_type);
CREATE INDEX IF NOT EXISTS idx_listings_property_type ON listings(property_type);
CREATE INDEX IF NOT EXISTS idx_listings_featured ON listings(featured);

-- 3. Table: assets
CREATE TABLE IF NOT EXISTS assets (
    id VARCHAR(255) PRIMARY KEY,
    entity_type VARCHAR(100) NOT NULL,
    entity_id VARCHAR(255) NOT NULL,
    object_key TEXT NOT NULL,
    content_type VARCHAR(100) NOT NULL,
    file_size BIGINT NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'confirmed',
    is_public BOOLEAN NOT NULL DEFAULT true,
    owner_id VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_assets_entity ON assets(entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_assets_owner ON assets(owner_id);
CREATE INDEX IF NOT EXISTS idx_assets_status ON assets(status);
CREATE INDEX IF NOT EXISTS idx_assets_deleted_at ON assets(deleted_at);

-- 4. Table: listing_visits
CREATE TABLE IF NOT EXISTS listing_visits (
    id VARCHAR(255) PRIMARY KEY,
    visitor_id VARCHAR(255) NOT NULL,
    listing_id VARCHAR(255) NOT NULL,
    event_type VARCHAR(50) NOT NULL,
    source VARCHAR(50) NOT NULL DEFAULT 'public',
    duration_ms INTEGER,
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_visits_visitor_id ON listing_visits(visitor_id);
CREATE INDEX IF NOT EXISTS idx_visits_listing_id ON listing_visits(listing_id);
CREATE INDEX IF NOT EXISTS idx_visits_created_at ON listing_visits(created_at);
CREATE INDEX IF NOT EXISTS idx_visits_pairing ON listing_visits(listing_id, visitor_id, event_type);

