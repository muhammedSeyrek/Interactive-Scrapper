

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