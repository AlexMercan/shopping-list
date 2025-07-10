CREATE TABLE IF NOT EXISTS shopping_lists (
                                              id BIGSERIAL PRIMARY KEY,
                                              name VARCHAR(255) NOT NULL,
                                              created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
                                              version BIGINT NOT NULL DEFAULT 1,
                                              CONSTRAINT shopping_lists_version_positive CHECK (version > 0)
);

CREATE TABLE IF NOT EXISTS grocery_items (
                                             id BIGSERIAL PRIMARY KEY,
                                             list_id BIGINT NOT NULL REFERENCES shopping_lists(id),
                                             name VARCHAR(255) NOT NULL,
                                             quantity DECIMAL(10,3) NOT NULL DEFAULT 1,
                                             completed BOOLEAN NOT NULL DEFAULT FALSE,
                                             created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
                                             version BIGINT NOT NULL DEFAULT 1,
                                             CONSTRAINT grocery_items_version_positive CHECK (version > 0)
);


CREATE INDEX IF NOT EXISTS idx_grocery_items_list_id ON grocery_items(list_id);