-- +goose Up
-- Повторный backfill external_user_attributions.tracking_link_id из клика
-- (как в 00022): kazik-импортёр и старые пути вставки могли оставить строки с
-- NULL tracking_link_id при существующем tracking_click_id. После этого
-- history-запросы (HistoryAttributionsByLink / HistoryConversionsByLink)
-- могут фильтровать напрямую по a.tracking_link_id, используя индекс
-- attributions_link_firstseen_idx, вместо COALESCE с коррелированным
-- подзапросом в tracking_clicks, который вынуждал полный скан таблицы.

UPDATE external_user_attributions a
SET tracking_link_id = c.tracking_link_id
FROM tracking_clicks c
WHERE c.id = a.tracking_click_id
  AND a.tracking_link_id IS NULL
  AND c.tracking_link_id IS NOT NULL;

-- +goose Down
-- Нет дампа значений для отката; обратно в NULL безопасно не возвращать.
