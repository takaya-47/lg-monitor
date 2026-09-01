-- +goose Up
CREATE TABLE municipalities (
    id            INT UNSIGNED     NOT NULL AUTO_INCREMENT,
    prefecture_id TINYINT UNSIGNED NOT NULL,
    code          CHAR(6)          NOT NULL COMMENT '全国地方公共団体コード',
    name          VARCHAR(255)     NOT NULL COMMENT '市区町村名',
    created_at    DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY municipalities_code_uq (code),
    KEY municipalities_prefecture_id_idx (prefecture_id),
    CONSTRAINT municipalities_prefecture_id_fk
        FOREIGN KEY (prefecture_id) REFERENCES prefectures (id)
) ENGINE = InnoDB COMMENT = '市区町村マスタ';

-- +goose Down
DROP TABLE municipalities;
