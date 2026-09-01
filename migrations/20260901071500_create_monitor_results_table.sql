-- +goose Up
CREATE TABLE monitor_results (
    id                INT UNSIGNED      NOT NULL AUTO_INCREMENT,
    monitor_target_id INT UNSIGNED      NOT NULL,
    checked_at        DATETIME          NOT NULL COMMENT 'リクエスト時刻',
    is_success        BOOLEAN           NOT NULL COMMENT '監視上の成功／失敗の判定結果',
    status_code       SMALLINT UNSIGNED NULL     COMMENT 'HTTPステータスコード。レスポンスを受け取れなかった場合はNULL。',
    response_time_ms  INT UNSIGNED      NULL     COMMENT 'リクエスト開始からレスポンス受領までのミリ秒。レスポンスを受け取れなかった場合はNULL。',
    error_message     VARCHAR(512)      NULL     COMMENT 'エラーの詳細。成功時はNULL。',
    created_at        DATETIME          NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME          NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    KEY monitor_results_monitor_target_id_idx (monitor_target_id),
    CONSTRAINT monitor_results_monitor_target_id_fk
        FOREIGN KEY (monitor_target_id) REFERENCES monitor_targets (id)
) ENGINE = InnoDB COMMENT = '監視結果';

-- +goose Down
DROP TABLE monitor_results;
