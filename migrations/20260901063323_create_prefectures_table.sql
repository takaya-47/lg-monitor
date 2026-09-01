-- +goose Up
CREATE TABLE prefectures (
    id         TINYINT UNSIGNED NOT NULL AUTO_INCREMENT,
    code       CHAR(2)          NOT NULL COMMENT '都道府県コード（JIS X 0401）',
    name       VARCHAR(255)     NOT NULL COMMENT '都道府県名',
    created_at DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY prefectures_code_uq (code),
    UNIQUE KEY prefectures_name_uq (name)
) ENGINE = InnoDB COMMENT = '都道府県マスタ';

-- +goose Down
DROP TABLE prefectures;
