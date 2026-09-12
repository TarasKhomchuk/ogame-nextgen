-- core/migrations/0001_init_game_schema.sql
-- Structural initialization script for OGame Next-Gen database architecture

-- Table 1: System Users & Administration Entities
CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(20) NOT NULL DEFAULT 'PLAYER', -- 'PLAYER', 'MODERATOR', 'SUPERADMIN'
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Table 2: Planetary Space Core Infrastructure
CREATE TABLE IF NOT EXISTS planets (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(50) NOT NULL DEFAULT 'Homeworld',
    galaxy INT NOT NULL,
    system INT NOT NULL,
    position INT NOT NULL,
    metal_mine_level INT NOT NULL DEFAULT 0,
    crystal_mine_level INT NOT NULL DEFAULT 0,
    deuterium_synthesizer_level INT NOT NULL DEFAULT 0,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(galaxy, system, position) -- Orbital positioning collision guard lock
);
