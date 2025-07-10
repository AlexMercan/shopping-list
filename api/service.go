package api

import (
	"context"
	"errors"
	"log"
)

const staleClientStateError = "STALE_CLIENT_STATE"
const staleClientStateErrorMessage = "Client state is stale"

type ShoppingListService struct {
	repo ShoppingListEntityRepository
}

func NewShoppingListService(repo *ShoppingListEntityRepository) ShoppingListService {
	return ShoppingListService{
		repo: *repo,
	}
}

var _ StrictServerInterface = (*ShoppingListService)(nil)

// Get all shopping lists
func (service ShoppingListService) GetShoppingLists(ctx context.Context, request GetShoppingListsRequestObject) (GetShoppingListsResponseObject, error) {
	shoppingListEntities, err := service.repo.GetShoppingLists(ctx)
	if err != nil {
		return nil, err
	}

	var shoppingLists GetShoppingLists200JSONResponse = make([]ShoppingList, 0, len(shoppingListEntities))

	for _, entity := range shoppingListEntities {
		shoppingLists = append(shoppingLists, *getShoppingListModelFromEntity(entity))
	}

	return shoppingLists, nil
}

// Create a new shopping list
func (service ShoppingListService) CreateShoppingList(ctx context.Context, request CreateShoppingListRequestObject) (CreateShoppingListResponseObject, error) {
	entity, err := service.repo.CreateShoppingListEntity(ctx, request.Body.Name)
	if err != nil {
		return nil, err
	}

	return CreateShoppingList201JSONResponse(*getShoppingListModelFromEntity(entity)), nil
}

// Delete an existing shopping list
func (service ShoppingListService) DeleteShoppingList(ctx context.Context, request DeleteShoppingListRequestObject) (DeleteShoppingListResponseObject, error) {
	err := service.repo.DeleteShoppingListEntity(ctx, request.ListId)
	if err != nil {
		return nil, err
	}

	return DeleteShoppingList204Response{}, nil
}

// Add a grocery item to a shopping list
func (service ShoppingListService) AddGroceryItem(ctx context.Context, request AddGroceryItemRequestObject) (AddGroceryItemResponseObject, error) {
	entity, err := service.repo.CreateGroceryItemEntity(ctx, request.ListId, request.Body.Name, request.Body.Quantity)
	if err != nil {
		return nil, err
	}

	return AddGroceryItem201JSONResponse(*getGroceryModelFromEntity(entity)), nil
}

// Update grocery item belonging to a shopping list
func (service ShoppingListService) UpdateGroceryItem(ctx context.Context, request UpdateGroceryItemRequestObject) (UpdateGroceryItemResponseObject, error) {
	entity, err := service.repo.UpdateGroceryItemEntity(ctx, request.ItemId, request.ListId, *request.Body.Name, *request.Body.Quantity, request.Body.Version)
	if err != nil {
		var lockErr OptimisticLockError
		if errors.As(err, &lockErr) {
			log.Printf("Stale client state with when updating grocery item err: %v", lockErr)
			return UpdateGroceryItem409JSONResponse{createConflictResponseFromErr(lockErr)}, nil
		}

		return nil, err
	}

	return UpdateGroceryItem200JSONResponse(*getGroceryModelFromEntity(entity)), nil
}

// Set the "completed" flag on a shopping list item
func (service ShoppingListService) ToggleGroceryItem(ctx context.Context, request ToggleGroceryItemRequestObject) (ToggleGroceryItemResponseObject, error) {
	entity, err := service.repo.ToggleGroceryItemEntity(ctx, request.ItemId, request.ListId, request.Body.Version)
	if err != nil {
		var lockErr OptimisticLockError
		if errors.As(err, &lockErr) {
			log.Printf("Stale client state detected when updating grocery item toggle toggle: %v", lockErr)
			return ToggleGroceryItem409JSONResponse{createConflictResponseFromErr(lockErr)}, nil
		}
		return nil, err
	}

	return ToggleGroceryItem200JSONResponse(*getGroceryModelFromEntity(entity)), nil
}

func getShoppingListModelFromEntity(entity *ShoppingListEntity) *ShoppingList {
	if entity == nil {
		return nil
	}

	shoppingList := &ShoppingList{
		Id:            entity.ID,
		Name:          entity.Name,
		Version:       entity.Version,
		ShoppingItems: getGroceryModelsFromList(entity.ShoppingItems),
	}

	return shoppingList
}

func getGroceryModelsFromList(entities []GroceryItemEntity) *[]GroceryItem {
	items := make([]GroceryItem, 0, len(entities))
	for _, entity := range entities {
		items = append(items, *getGroceryModelFromEntity(&entity))
	}

	return &items
}

func getGroceryModelFromEntity(entity *GroceryItemEntity) *GroceryItem {
	if entity == nil {
		return nil
	}

	return &GroceryItem{
		Id:        entity.ID,
		Name:      entity.Name,
		Completed: entity.Completed,
		Quantity:  entity.Quantity,
		Version:   entity.Version,
	}
}

func createConflictResponseFromErr(err OptimisticLockError) ConflictJSONResponse {
	return ConflictJSONResponse{
		CurrentVersion: err.CurrentVersion,
		Error:          staleClientStateError,
		Message:        staleClientStateErrorMessage,
	}
}
