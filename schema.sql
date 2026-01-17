

CREATE TABLE IF NOT EXISTS dark_web_contents (
    id SERIAL PRIMARY KEY,                 
    source_name VARCHAR(255) NOT NULL,      
    source_url TEXT NOT NULL,               
    content TEXT NOT NULL,                  
    title VARCHAR(255),                     
    published_date TIMESTAMP,              
    criticality_score INT DEFAULT 0,        
    category VARCHAR(100),                 
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP 
);

CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    role VARCHAR(20) DEFAULT 'admin', 
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS entities (
    id SERIAL PRIMARY KEY,
    content_id INT REFERENCES dark_web_contents(id) ON DELETE CASCADE,
    entity_type VARCHAR(50), -- ex: 'BTC_WALLET', 'EMAIL', 'PGP_KEY', 'GA_ID'
    entity_value TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- we have to enhance for db performance.
CREATE INDEX idx_entity_value ON entities(entity_value);