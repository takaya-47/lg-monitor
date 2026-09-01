-- +goose Up
CREATE TABLE monitor_targets (
    id              INT UNSIGNED NOT NULL AUTO_INCREMENT,
    municipality_id INT UNSIGNED NOT NULL,
    url             VARCHAR(255) NOT NULL COMMENT '監視対象URL',
    label           VARCHAR(255) NOT NULL DEFAULT '' COMMENT '同一自治体で複数URLを監視する場合の識別用ラベル',
    is_active       BOOLEAN      NOT NULL DEFAULT TRUE COMMENT '監視対象に含めるか否か',
    created_at      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY monitor_targets_url_uq (url),
    KEY monitor_targets_municipality_id_idx (municipality_id),
    CONSTRAINT monitor_targets_municipality_id_fk
        FOREIGN KEY (municipality_id) REFERENCES municipalities (id),
    CONSTRAINT monitor_targets_url_chk CHECK (url LIKE 'http%')
) ENGINE = InnoDB COMMENT = '監視対象URL';

-- +goose Down
DROP TABLE monitor_targets;
