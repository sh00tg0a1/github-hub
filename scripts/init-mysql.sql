CREATE DATABASE IF NOT EXISTS github_hub CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

USE github_hub;

CREATE TABLE IF NOT EXISTS github_events (
    id INT AUTO_INCREMENT PRIMARY KEY,
    event_id VARCHAR(36) NOT NULL UNIQUE,
    event_type VARCHAR(50) NOT NULL,
    event_status VARCHAR(50) NOT NULL,
    repository VARCHAR(255) NOT NULL,
    branch VARCHAR(255) NOT NULL,
    target_branch VARCHAR(255),
    commit_sha VARCHAR(255),
    pr_number INT,
    action VARCHAR(50),
    pusher VARCHAR(255),
    author VARCHAR(255),
    payload JSON,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    processed_at TIMESTAMP NULL,
    INDEX idx_event_id (event_id),
    INDEX idx_event_type (event_type),
    INDEX idx_event_status (event_status),
    INDEX idx_repository (repository)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS pr_quality_checks (
    id INT AUTO_INCREMENT PRIMARY KEY,
    github_event_id VARCHAR(36) NOT NULL,
    check_type VARCHAR(50) NOT NULL,
    check_status VARCHAR(50) NOT NULL,
    stage VARCHAR(50) NOT NULL,
    stage_order INT NOT NULL,
    check_order INT NOT NULL,
    started_at TIMESTAMP NULL,
    completed_at TIMESTAMP NULL,
    duration_seconds DOUBLE,
    error_message TEXT,
    output TEXT,
    retry_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_github_event_id (github_event_id),
    INDEX idx_check_type (check_type),
    INDEX idx_check_status (check_status),
    INDEX idx_stage (stage),
    FOREIGN KEY (github_event_id) REFERENCES github_events(event_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;