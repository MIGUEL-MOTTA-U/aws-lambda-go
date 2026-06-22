-- Database schema for users and listings tables in Neon Postgres

CREATE TABLE users (
    id VARCHAR(255) PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT NOT NULL,
    username TEXT NOT NULL,
    birthdate TEXT NOT NULL,
    creationdate TEXT NOT NULL
);

CREATE TABLE listings (
    listing_id VARCHAR(255) PRIMARY KEY,
    slug TEXT NOT NULL,
    url TEXT NOT NULL,
    language TEXT NOT NULL,
    title TEXT NOT NULL,
    property_type TEXT NOT NULL,
    subtype TEXT,
    operation_type TEXT NOT NULL,
    publication_status TEXT NOT NULL,
    location JSONB NOT NULL,
    pricing JSONB NOT NULL,
    areas JSONB NOT NULL,
    layout JSONB NOT NULL,
    structure JSONB NOT NULL,
    features JSONB NOT NULL,
    media JSONB NOT NULL,
    commercial JSONB NOT NULL,
    metadata JSONB NOT NULL
);
