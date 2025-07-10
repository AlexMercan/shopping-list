package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ShoppingListEntity struct {
	ID            int64               `json:"id" db:"id"`
	Name          string              `json:"name" db:"name"`
	CreatedAt     time.Time           `json:"createdAt" db:"created_at"`
	Version       int64               `json:"version" db:"version"`
	ShoppingItems []GroceryItemEntity `json:"shoppingItems,omitempty"`
}

type GroceryItemEntity struct {
	ID        int64     `json:"id" db:"id"`
	ListID    int64     `json:"listId" db:"list_id"`
	Name      string    `json:"name" db:"name"`
	Quantity  int32     `json:"quantity" db:"quantity"`
	Completed bool      `json:"completed" db:"completed"`
	CreatedAt time.Time `json:"createdAt" db:"created_at"`
	Version   int64     `json:"version" db:"version"`
}

type OptimisticLockError struct {
	ResourceType    string
	ResourceID      int64
	CurrentVersion  int64
	ProvidedVersion int64
}

func (e OptimisticLockError) Error() string {
	return fmt.Sprintf("optimistic lock failed for %s %d: current version %d, provided version %d",
		e.ResourceType, e.ResourceID, e.CurrentVersion, e.ProvidedVersion)
}

type NotFoundError struct {
	ResourceType string
	ResourceID   int64
}

func (e NotFoundError) Error() string {
	return fmt.Sprintf("%s with id %d not found", e.ResourceType, e.ResourceID)
}

type ShoppingListEntityRepository interface {
	CreateShoppingListEntity(ctx context.Context, name string) (*ShoppingListEntity, error)
	GetShoppingLists(ctx context.Context) ([]*ShoppingListEntity, error)
	DeleteShoppingListEntity(ctx context.Context, listID int64) error
	CreateGroceryItemEntity(ctx context.Context, listID int64, name string, quantity int32) (*GroceryItemEntity, error)
	UpdateGroceryItemEntity(ctx context.Context, itemID int64, listID int64, name string, quantity int32, version int64) (*GroceryItemEntity, error)
	ToggleGroceryItemEntity(ctx context.Context, itemID int64, listID int64, version int64) (*GroceryItemEntity, error)
}

type ShoppingListRepository struct {
	db *pgxpool.Pool
}

func NewShoppingListRepository(db *pgxpool.Pool) *ShoppingListRepository {
	return &ShoppingListRepository{db: db}
}

const (
	createShoppingListEntityQuery = `
		INSERT INTO shopping_lists (name, created_at, version)
		VALUES ($1, NOW(), 1)
		RETURNING id, name, created_at, version`
	getShoppingListsQuery = `
		SELECT 
			sl.id, sl.name, sl.created_at, sl.version,
			gi.id, gi.name, gi.quantity, gi.completed, gi.created_at, gi.version
		FROM shopping_lists sl
		LEFT JOIN grocery_items gi ON sl.id = gi.list_id`

	getShoppingListEntityByIDQuery = `
		SELECT id, name, created_at, version
		FROM shopping_lists
		WHERE id = $1`

	deleteShoppingListEntityQuery = `
		DELETE FROM shopping_lists where id = $1`
	deleteGroceryItemEntitiesByListQuery = `
		DELETE FROM grocery_items where list_id = $1`

	createGroceryItemEntityQuery = `
		INSERT INTO grocery_items (list_id, name, quantity, completed, created_at, version)
		VALUES ($1, $2, $3, false, NOW(), 1)
		RETURNING id, list_id, name, quantity, completed, created_at, version`

	updateGroceryItemEntityQuery = `
		UPDATE grocery_items
		SET name = $1, quantity = $2, version = version + 1
		WHERE id = $3 AND list_id = $4 AND version = $5
		AND EXISTS (SELECT 1 FROM shopping_lists WHERE id = $4)
		RETURNING id, list_id, name, quantity, completed, created_at, version`

	toggleGroceryItemEntityQuery = `
		UPDATE grocery_items
		SET completed = NOT completed, version = version + 1
		WHERE id = $1 AND list_id = $2 AND version = $3
		AND EXISTS (SELECT 1 FROM shopping_lists WHERE id = $2)
		RETURNING id, list_id, name, quantity, completed, created_at, version`

	getCurrentGroceryItemEntityVersionQuery = `
		SELECT version 
		FROM grocery_items
		WHERE id = $1 AND list_id = $2`
)

func (r *ShoppingListRepository) CreateShoppingListEntity(ctx context.Context, name string) (*ShoppingListEntity, error) {
	var list ShoppingListEntity
	err := r.db.QueryRow(ctx, createShoppingListEntityQuery, name).Scan(
		&list.ID, &list.Name, &list.CreatedAt, &list.Version,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create shopping list: %w", err)
	}
	return &list, nil
}

func (r *ShoppingListRepository) GetShoppingLists(ctx context.Context) ([]*ShoppingListEntity, error) {
	rows, err := r.db.Query(ctx, getShoppingListsQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to get shopping lists: %w", err)
	}
	defer rows.Close()

	// Map to store lists by ID to group items
	listsMap := make(map[int64]*ShoppingListEntity)
	var orderedListIDs []int64

	for rows.Next() {
		var (
			// Shopping list fields
			listId, listVersion int64
			listName            *string
			listCreatedAt       time.Time

			// Grocery item fields (nullable for LEFT JOIN)
			itemId, itemVersion *int64
			itemName            *string
			itemQuantity        *int32
			itemCompleted       *bool
			itemCreatedAt       *time.Time
		)

		err := rows.Scan(
			&listId, &listName, &listCreatedAt, &listVersion,
			&itemId, &itemName, &itemQuantity, &itemCompleted, &itemCreatedAt, &itemVersion,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Get or create the shopping list
		list, exists := listsMap[listId]
		if !exists {
			list = &ShoppingListEntity{
				ID:            listId,
				Name:          *listName,
				CreatedAt:     listCreatedAt,
				Version:       listVersion,
				ShoppingItems: []GroceryItemEntity{},
			}
			listsMap[listId] = list
			orderedListIDs = append(orderedListIDs, listId)
		}

		// Add item if it exists (not null from LEFT JOIN)
		if itemId != nil {
			item := GroceryItemEntity{
				ID:        *itemId,
				ListID:    listId,
				Name:      *itemName,
				Quantity:  *itemQuantity,
				Completed: *itemCompleted,
				CreatedAt: *itemCreatedAt,
				Version:   *itemVersion,
			}
			list.ShoppingItems = append(list.ShoppingItems, item)
		}
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating shopping lists: %w", err)
	}

	var lists []*ShoppingListEntity
	for _, listID := range orderedListIDs {
		lists = append(lists, listsMap[listID])
	}

	return lists, nil
}

func (r *ShoppingListRepository) GetShoppingListEntityByID(ctx context.Context, listID int64) (*ShoppingListEntity, error) {
	var list ShoppingListEntity
	err := r.db.QueryRow(ctx, getShoppingListEntityByIDQuery, listID).Scan(
		&list.ID, &list.Name, &list.CreatedAt, &list.Version,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, NotFoundError{ResourceType: "shopping list", ResourceID: listID}
		}
		return nil, fmt.Errorf("failed to get shopping list: %w", err)
	}
	return &list, nil
}

func (r *ShoppingListRepository) DeleteShoppingListEntity(ctx context.Context, listID int64) error {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func(tx pgx.Tx, ctx context.Context) {
		err := tx.Rollback(ctx)
		if err != nil {
			log.Printf("failed to rollback transaction: %v", err)
		}
	}(tx, ctx)

	// First delete all items in the list
	_, err = tx.Exec(ctx, deleteGroceryItemEntitiesByListQuery, listID)
	if err != nil {
		return fmt.Errorf("failed to delete grocery items: %w", err)
	}

	// Then delete the list
	result, err := tx.Exec(ctx, deleteShoppingListEntityQuery, listID)
	if err != nil {
		return fmt.Errorf("failed to delete shopping list: %w", err)
	}

	rowsAffected := result.RowsAffected()

	if rowsAffected == 0 {
		return NotFoundError{ResourceType: "shopping list", ResourceID: listID}
	}

	return tx.Commit(ctx)
}

func (r *ShoppingListRepository) CreateGroceryItemEntity(ctx context.Context, listID int64, name string, quantity int32) (*GroceryItemEntity, error) {
	// First verify the shopping list exists
	_, err := r.GetShoppingListEntityByID(ctx, listID)
	if err != nil {
		return nil, err
	}

	var item GroceryItemEntity
	err = r.db.QueryRow(ctx, createGroceryItemEntityQuery, listID, name, quantity).Scan(
		&item.ID, &item.ListID, &item.Name, &item.Quantity, &item.Completed, &item.CreatedAt, &item.Version,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create grocery item: %w", err)
	}
	return &item, nil
}

func (r *ShoppingListRepository) UpdateGroceryItemEntity(ctx context.Context, itemID int64, listID int64, name string, quantity int32, version int64) (*GroceryItemEntity, error) {
	var item GroceryItemEntity
	err := r.db.QueryRow(ctx, updateGroceryItemEntityQuery, name, quantity, itemID, listID, version).Scan(
		&item.ID, &item.ListID, &item.Name, &item.Quantity, &item.Completed, &item.CreatedAt, &item.Version,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Check if it's an optimistic lock error or not found
			var currentVersion int64
			versionErr := r.db.QueryRow(ctx, getCurrentGroceryItemEntityVersionQuery, itemID, listID).Scan(&currentVersion)
			if errors.Is(versionErr, sql.ErrNoRows) {
				return nil, NotFoundError{ResourceType: "grocery item", ResourceID: itemID}
			} else if versionErr == nil {
				return nil, OptimisticLockError{
					ResourceType:    "grocery item",
					ResourceID:      itemID,
					CurrentVersion:  currentVersion,
					ProvidedVersion: version,
				}
			}
			return nil, fmt.Errorf("failed to check version: %w", versionErr)
		}
		return nil, fmt.Errorf("failed to update grocery item: %w", err)
	}
	return &item, nil
}

func (r *ShoppingListRepository) ToggleGroceryItemEntity(ctx context.Context, itemID int64, listID int64, version int64) (*GroceryItemEntity, error) {
	var item GroceryItemEntity
	err := r.db.QueryRow(ctx, toggleGroceryItemEntityQuery, itemID, listID, version).Scan(
		&item.ID, &item.ListID, &item.Name, &item.Quantity, &item.Completed, &item.CreatedAt, &item.Version,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Check if it's an optimistic lock error or not found
			var currentVersion int64
			versionErr := r.db.QueryRow(ctx, getCurrentGroceryItemEntityVersionQuery, itemID, listID).Scan(&currentVersion)
			if errors.Is(versionErr, sql.ErrNoRows) {
				return nil, NotFoundError{ResourceType: "grocery item", ResourceID: itemID}
			} else if versionErr == nil {
				return nil, OptimisticLockError{
					ResourceType:    "grocery item",
					ResourceID:      itemID,
					CurrentVersion:  currentVersion,
					ProvidedVersion: version,
				}
			}
			return nil, fmt.Errorf("failed to check version: %w", versionErr)
		}
		return nil, fmt.Errorf("failed to toggle grocery item: %w", err)
	}
	return &item, nil
}
