CREATE TABLE IF NOT EXISTS repository (
    id MEDIUMINT NOT NULL AUTO_INCREMENT COMMENT 'repo id',
    username CHAR(30) NOT NULL COMMENT 'create username',
    name CHAR(30) NOT NULL COMMENT 'name',
    private BOOLEAN NOT NULL DEFAULT FALSE COMMENT 'is private',
    os_type VARCHAR(50) NOT NULL DEFAULT '' COMMENT 'os type',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'repo create time',
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'repo update time',
    PRIMARY KEY (id),
    UNIQUE (username, name)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `image` (
    id MEDIUMINT NOT NULL AUTO_INCREMENT COMMENT 'tag id',
    repo_id MEDIUMINT NOT NULL COMMENT 'repo id',
    tag VARCHAR(40) NOT NULL COMMENT 'image tag',
    labels JSON NOT NULL COMMENT 'image labels',
    # state ENUM( 'creating', 'ready', 'unknown') NOT NULL DEFAULT 'creating' COMMENT 'image state',
    digest VARCHAR(80) NOT NULL COMMENT 'image digest',
    size BIGINT(20) UNSIGNED NOT NULL DEFAULT '0' COMMENT 'image size, byte',
    virtual_size BIGINT(20) UNSIGNED NOT NULL DEFAULT '0' COMMENT 'image virtual size, byte',
    FORMAT VARCHAR(10) NOT NULL COMMENT 'image format',
    os JSON NOT NULL COMMENT 'os information',
    snapshot VARCHAR(80) NOT NULL COMMENT 'RBD snapshot for image',
    description VARCHAR(100) NOT NULL DEFAULT '' COMMENT 'image description',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'image create time',
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'image update time',
    PRIMARY KEY (id),
    UNIQUE (repo_id, tag)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `user` (
    id INT(10) UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'user id',
    username VARCHAR(50) NOT NULL COMMENT 'user name',
    password VARCHAR(255) NOT NULL COMMENT 'user pwd',
    email VARCHAR(100) NOT NULL COMMENT 'user email',
    nickname VARCHAR(50) NOT NULL COMMENT 'nick name',
    admin BOOLEAN NOT NULL DEFAULT FALSE COMMENT 'is administrator',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'create time',
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'update time',
    PRIMARY KEY (id),
    UNIQUE KEY username (username),
    UNIQUE KEY namespace (namespace)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS private_token (
    id INT(10) UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'user id',
    name VARCHAR(50) NOT NULL COMMENT 'token name',
    user_id MEDIUMINT NOT NULL COMMENT 'user id',
    token VARCHAR(100) NOT NULL COMMENT 'token',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'create time',
    last_used TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'last use time',
    expired_at DATETIME NOT NULL DEFAULT '9999-12-31 23:59:59' COMMENT 'expired time',
    PRIMARY KEY (id),
    UNIQUE KEY token (token),
    UNIQUE (user_id, name)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;
